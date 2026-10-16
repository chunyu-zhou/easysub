package notify

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net"
	"net/mail"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"easysub/internal/config"
)

type tlsMode int

const (
	tlsImplicit      tlsMode = iota // SMTP_SSL
	tlsStartOptional                // 服务器支持时才 STARTTLS
	tlsStartRequired                // 必须 STARTTLS
)

const smtpTimeout = 20 * time.Second

type mailJob struct {
	host, user, pass string
	port             int
	mode             tlsMode
	from             string
	to               []string
	subject, body    string
}

// ---- 用户 Email 渠道 ---- //

func sendEmail(ctx context.Context, conf map[string]any, subject, text string) error {
	host := str(conf, "host")
	to := splitCSV(str(conf, "to"))
	sender := str(conf, "from")
	if sender == "" {
		sender = str(conf, "username")
	}
	if host == "" || len(to) == 0 || sender == "" {
		return errors.New("Email 未配置 SMTP 主机 / 发件人 / 收件人")
	}
	useSSL := boolean(conf, "ssl")
	defPort := 587.0
	mode := tlsStartOptional
	if useSSL {
		defPort, mode = 465, tlsImplicit
	}
	if subject == "" {
		subject = defaultNotifyTitle
	}
	return deliver(ctx, mailJob{
		host: host, port: int(num(conf, "port", defPort)), mode: mode,
		user: str(conf, "username"), pass: str(conf, "password"),
		from: sender, to: to, subject: subject, body: text,
	})
}

// ---- 系统 SMTP（注册验证码 / 密码重置） ---- //

// SMTPConfigured 是否配置了系统 SMTP；未配置时注册无需邮箱验证。
func SMTPConfigured() bool { return config.C.SMTPHost != "" && config.C.SMTPFrom != "" }

func sendSystem(to, subject, body string) error {
	if !SMTPConfigured() {
		return errors.New("未配置 SMTP")
	}
	c := config.C
	mode := tlsImplicit
	if c.SMTPTLS {
		mode = tlsStartRequired
	}
	return deliver(context.Background(), mailJob{
		host: c.SMTPHost, port: c.SMTPPort, mode: mode, user: c.SMTPUser, pass: c.SMTPPassword,
		from: c.SMTPFrom, to: []string{to}, subject: subject, body: body,
	})
}

// SendCode 发送注册验证码。
func SendCode(to, code string) error {
	return sendSystem(to, "省心订阅 EasySub — 注册验证码",
		"你好，\n\n你的注册验证码是："+code+"\n\n"+
			"该验证码 10 分钟内有效。如果不是你本人操作，请忽略此邮件。\n\n— 省心订阅 EasySub")
}

// SendReset 发送密码重置码。
func SendReset(to, code string) error {
	return sendSystem(to, "省心订阅 EasySub — 密码重置码",
		"你好，\n\n你的密码重置码是："+code+"\n\n"+
			"该验证码 15 分钟内有效。如果不是你本人操作，请忽略此邮件并注意账号安全。\n\n— 省心订阅 EasySub")
}

// ---- SMTP 投递 ---- //

// addrOf 解析 "Name <a@b>" 形式；返回信封地址与编码后的头部值。
func addrOf(s string) (envelope, header string) {
	if a, err := mail.ParseAddress(s); err == nil {
		return a.Address, a.String()
	}
	return s, s
}

func buildMessage(j mailJob) []byte {
	_, fromHdr := addrOf(j.from)
	toHdrs := make([]string, len(j.to))
	for i, t := range j.to {
		_, toHdrs[i] = addrOf(t)
	}
	var b strings.Builder
	b.WriteString("From: " + fromHdr + "\r\n")
	b.WriteString("To: " + strings.Join(toHdrs, ", ") + "\r\n")
	b.WriteString("Subject: " + mime.BEncoding.Encode("UTF-8", j.subject) + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	b.WriteString("Content-Transfer-Encoding: base64\r\n\r\n")
	enc := base64.StdEncoding.EncodeToString([]byte(j.body))
	for len(enc) > 76 {
		b.WriteString(enc[:76] + "\r\n")
		enc = enc[76:]
	}
	b.WriteString(enc + "\r\n")
	return []byte(b.String())
}

func deliver(ctx context.Context, j mailJob) (err error) {
	addr := net.JoinHostPort(j.host, strconv.Itoa(j.port))
	tlsConf := &tls.Config{ServerName: j.host}
	dialer := &net.Dialer{Timeout: smtpTimeout}
	var conn net.Conn
	if j.mode == tlsImplicit {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConf}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return fmt.Errorf("SMTP 连接失败：%w", err)
	}
	// 整个会话的读写超时
	_ = conn.SetDeadline(time.Now().Add(3 * smtpTimeout))
	c, err := smtp.NewClient(conn, j.host)
	if err != nil {
		conn.Close()
		return fmt.Errorf("SMTP 握手失败：%w", err)
	}
	defer c.Close()

	if j.mode != tlsImplicit {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsConf); err != nil {
				return fmt.Errorf("SMTP STARTTLS 失败：%w", err)
			}
		} else if j.mode == tlsStartRequired {
			return errors.New("SMTP 服务器不支持 STARTTLS")
		}
	}
	if j.user != "" {
		ok, mechs := c.Extension("AUTH")
		if !ok {
			return errors.New("SMTP 服务器不支持 AUTH 登录")
		}
		if err := c.Auth(pickAuth(mechs, j.user, j.pass)); err != nil {
			return fmt.Errorf("SMTP 登录失败：%w", err)
		}
	}
	fromAddr, _ := addrOf(j.from)
	if err := c.Mail(fromAddr); err != nil {
		return fmt.Errorf("SMTP 发件人被拒绝：%w", err)
	}
	for _, t := range j.to {
		rcpt, _ := addrOf(t)
		if err := c.Rcpt(rcpt); err != nil {
			return fmt.Errorf("SMTP 收件人 %s 被拒绝：%w", rcpt, err)
		}
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("SMTP DATA 失败：%w", err)
	}
	if _, err := w.Write(buildMessage(j)); err != nil {
		w.Close()
		return fmt.Errorf("SMTP 写入失败：%w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("SMTP 发送失败：%w", err)
	}
	return c.Quit()
}

// pickAuth 按服务器声明选择认证方式（与 smtplib.login 一致：CRAM-MD5 > PLAIN > LOGIN）。
// 不做 net/smtp 的"非 TLS 拒绝 PLAIN"限制，与原版行为保持一致。
func pickAuth(mechs, user, pass string) smtp.Auth {
	has := map[string]bool{}
	for _, m := range strings.Fields(strings.ToUpper(mechs)) {
		has[m] = true
	}
	switch {
	case has["CRAM-MD5"]:
		return smtp.CRAMMD5Auth(user, pass)
	case has["PLAIN"]:
		return plainAuth{user, pass}
	case has["LOGIN"]:
		return &loginAuth{user: user, pass: pass}
	default:
		return plainAuth{user, pass}
	}
}

type plainAuth struct{ user, pass string }

func (a plainAuth) Start(*smtp.ServerInfo) (string, []byte, error) {
	return "PLAIN", []byte("\x00" + a.user + "\x00" + a.pass), nil
}

func (a plainAuth) Next(_ []byte, more bool) ([]byte, error) {
	if more {
		return nil, errors.New("unexpected server challenge")
	}
	return nil, nil
}

type loginAuth struct {
	user, pass string
	step       int
}

func (a *loginAuth) Start(*smtp.ServerInfo) (string, []byte, error) { return "LOGIN", nil, nil }

func (a *loginAuth) Next(_ []byte, more bool) ([]byte, error) {
	if !more {
		return nil, nil
	}
	a.step++
	switch a.step {
	case 1:
		return []byte(a.user), nil
	case 2:
		return []byte(a.pass), nil
	}
	return nil, errors.New("unexpected server challenge")
}
