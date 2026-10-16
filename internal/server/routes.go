package server

import (
	"net/http"

	"easysub/internal/icons"
)

// routes registers API endpoints before static and SPA fallbacks.
func (s *Server) routes() {
	s.handle("GET /healthz", public, s.health)
	s.handle("GET /readyz", withDB, s.ready)

	s.handle("POST /api/auth/login", withDB, s.login)
	s.handle("POST /api/auth/register", withDB, s.register)
	s.handle("POST /api/auth/refresh", withDB, s.refresh)
	s.handle("POST /api/auth/verify-email", withDB, s.verifyEmail)
	s.handle("POST /api/auth/forgot", withDB, s.forgotPassword)
	s.handle("POST /api/auth/reset", withDB, s.resetPassword)
	s.handle("GET /api/auth/me", authed, s.me)

	s.handle("PATCH /api/me", authed, s.updateMe)
	s.handle("PATCH /api/me/account", authed, s.updateAccount)
	s.handle("POST /api/me/password", authed, s.changePassword)
	s.handle("GET /api/me/tokens", authed, s.listTokens)
	s.handle("POST /api/me/tokens", authed, s.createToken)
	s.handle("DELETE /api/me/tokens/{id}", authed, s.deleteToken)

	s.handle("GET /api/subscriptions", authed, s.listSubscriptions)
	s.handle("POST /api/subscriptions", authed, s.createSubscription)
	s.handle("PUT /api/subscriptions/{id}", authed, s.updateSubscription)
	s.handle("DELETE /api/subscriptions/{id}", authed, s.deleteSubscription)
	s.handle("POST /api/subscriptions/{id}/renew", authed, s.renewSubscription)
	s.handle("POST /api/subscriptions/reorder", authed, s.reorderSubscriptions)
	s.handle("GET /api/subscriptions/export.csv", authed, s.exportSubscriptionsCSV)
	s.handle("POST /api/subscriptions/import-csv", authed, s.importSubscriptionsCSV)

	s.handle("GET /api/categories", authed, s.listCategories)
	s.handle("GET /api/bundles", authed, s.listBundles)
	s.handle("POST /api/bundles", authed, s.createBundle)
	s.handle("DELETE /api/bundles/{id}", authed, s.deleteBundle)

	s.handle("GET /api/currencies", authed, s.listCurrencies)
	s.handle("GET /api/currencies/catalog", authed, s.currencyCatalog)
	s.handle("POST /api/currencies", authed, s.createCurrency)
	s.handle("PUT /api/currencies/{code}", authed, s.updateCurrency)
	s.handle("DELETE /api/currencies/{code}", authed, s.deleteCurrency)
	s.handle("POST /api/currencies/reorder", authed, s.reorderCurrencies)
	s.handle("POST /api/currencies/restore-defaults", authed, s.restoreCurrencies)
	s.handle("GET /api/currencies/rates", authed, s.currencyRates)
	s.handle("GET /api/currencies/rate-table", authed, s.currencyRateTable)
	s.handle("POST /api/currencies/rates/refresh", authed, s.refreshRates)
	s.handle("POST /api/currencies/rates/auto-refresh", authed, s.autoRefreshRates)

	s.handle("GET /api/payment-methods", authed, s.listPaymentMethods)
	s.handle("POST /api/payment-methods", authed, s.createPaymentMethod)
	s.handle("PUT /api/payment-methods/{id}", authed, s.updatePaymentMethod)
	s.handle("DELETE /api/payment-methods/{id}", authed, s.deletePaymentMethod)
	s.handle("POST /api/payment-methods/reorder", authed, s.reorderPaymentMethods)
	s.handle("POST /api/payment-methods/restore-defaults", authed, s.restorePaymentMethods)

	s.handle("GET /api/dashboard", authed, s.dashboard)
	s.handle("GET /api/reports/insights", authed, s.reportInsights)
	s.handle("GET /api/reports/expired", authed, s.reportExpired)
	s.handle("GET /api/reports/category-detail", authed, s.reportCategoryDetail)
	s.handle("GET /api/reports/ranking", authed, s.reportRanking)
	s.handle("GET /api/reports/one-time", authed, s.reportOneTime)
	s.handle("GET /api/reports/upcoming", authed, s.reportUpcoming)
	s.handle("GET /api/reports/recent-payments", authed, s.reportRecentPayments)
	s.handle("GET /api/reports/export.csv", authed, s.exportReportCSV)

	s.handle("GET /api/icons/library", authed, s.iconManifest)
	s.handle("POST /api/icons/from-url", authed, s.iconFromURL)
	s.handle("POST /api/icons/upload", authed, s.iconUpload)
	s.mux.Handle("GET /api/icons/library/{slug}", s.icons)
	s.mux.Handle("GET /static/icons/", http.StripPrefix("/static/icons/", staticDir(icons.UploadDir())))

	s.handle("GET /api/backup/export", authed, s.exportBackup)
	s.handle("POST /api/backup/import", authed, s.importBackup)
	s.handle("GET /api/backup/export-all", admin, s.exportAllBackup)
	s.handle("POST /api/backup/import-all", admin, s.importAllBackup)
	s.handle("GET /api/backup/auto", admin, s.listLocalBackups)
	s.handle("POST /api/backup/auto/run", admin, s.runLocalBackup)
	s.handle("GET /api/webdav/config", authed, s.webdavConfig)
	s.handle("PUT /api/webdav/config", authed, s.saveWebDAVConfig)
	s.handle("POST /api/webdav/test", authed, s.testWebDAV)
	s.handle("GET /api/webdav/files", authed, s.listWebDAVFiles)
	s.handle("POST /api/webdav/backup", authed, s.runWebDAVBackup)
	s.handle("POST /api/webdav/restore", authed, s.restoreWebDAVBackup)
	s.handle("DELETE /api/webdav/files/{name}", authed, s.deleteWebDAVFile)

	s.handle("GET /api/notifications/config", authed, s.notificationConfig)
	s.handle("PUT /api/notifications/config", authed, s.saveNotificationConfig)
	s.handle("POST /api/notifications/test", authed, s.testNotification)
	s.handle("POST /api/notifications/wecom/check", authed, s.checkWeCom)
	s.handle("GET /api/notifications/telegram/me", authed, s.telegramMe)
	s.handle("GET /api/notifications/telegram/updates", authed, s.telegramUpdates)
	s.handle("GET /api/notifications/logs", authed, s.notificationLogs)
	s.handle("POST /api/notifications/run-scan", authed, s.runNotificationScan)

	s.handle("GET /api/calendar/token", authed, s.calendarToken)
	s.handle("GET /api/calendar/{token}/easysub.ics", withDB, s.calendarICS)
	s.handle("GET /api/logs", authed, s.logs)
	s.handle("GET /api/system/info", authed, s.systemInfo)
	s.handle("GET /api/system/version-check", authed, s.versionCheck)
	s.handle("GET /api/docs", public, s.apiDocs)

	s.handle("GET /api/admin/users", admin, s.listUsers)
	s.handle("POST /api/admin/users", admin, s.createUser)
	s.handle("PATCH /api/admin/users/{id}", admin, s.updateUser)
	s.handle("DELETE /api/admin/users/{id}", admin, s.deleteUser)

	s.handle("/mcp", authed, s.mcp)
	s.mux.Handle("/", s.spa())
}
