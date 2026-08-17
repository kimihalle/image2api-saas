package service

import (
	"context"
	"crypto/tls"
	"errors"
	"html"
	"mime"
	"net"
	"net/smtp"
	"strconv"
	"strings"

	"backend/internal/repo"
)

type EmailTemplate struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

type SMTPTemplateSettings struct {
	Register       EmailTemplate `json:"register"`
	Reset          EmailTemplate `json:"reset"`
	Welcome        EmailTemplate `json:"welcome"`
	WelcomeEnabled bool          `json:"welcome_enabled"`
}

type SMTPConfig struct {
	Host          string
	Port          int
	Username      string
	Password      string
	FromAddr      string
	UseTLS        bool
	SiteName      string
	ExpireMinutes int
	Templates     SMTPTemplateSettings
}

type SMTPService struct{}

func loadSMTPTemplateSettings(ctx context.Context, settings *repo.SiteSettingRepository, siteName string) SMTPTemplateSettings {
	read := func(key, fallback string) string {
		value, _ := settings.GetValue(ctx, key)
		if strings.TrimSpace(value) == "" {
			return fallback
		}
		return value
	}
	defaults := defaultSMTPTemplates(siteName)
	return SMTPTemplateSettings{
		Register: EmailTemplate{
			Subject: read("smtp.template.register.subject", defaults.Register.Subject),
			HTML:    read("smtp.template.register.html", defaults.Register.HTML),
		},
		Reset: EmailTemplate{
			Subject: read("smtp.template.reset.subject", defaults.Reset.Subject),
			HTML:    read("smtp.template.reset.html", defaults.Reset.HTML),
		},
		Welcome: EmailTemplate{
			Subject: read("smtp.template.welcome.subject", defaults.Welcome.Subject),
			HTML:    read("smtp.template.welcome.html", defaults.Welcome.HTML),
		},
		WelcomeEnabled: parseBoolSetting(read("smtp.template.welcome.enabled", "false"), false),
	}
}

func defaultSMTPTemplates(siteName string) SMTPTemplateSettings {
	if strings.TrimSpace(siteName) == "" {
		siteName = "Vivid"
	}
	wrapper := func(title, lead, body string) string {
		return `<!doctype html><html><body style="margin:0;background:#f5f5f1;font-family:Arial,'Microsoft YaHei',sans-serif;color:#252a26"><div style="max-width:560px;margin:32px auto;background:#fff;border:1px solid #e3e4de;border-radius:8px;overflow:hidden"><div style="padding:24px 28px;border-bottom:1px solid #e3e4de;font-size:18px;font-weight:700">{{site_name}}</div><div style="padding:30px 28px"><h1 style="margin:0 0 12px;font-size:22px">` + title + `</h1><p style="margin:0 0 22px;color:#687068;line-height:1.7">` + lead + `</p>` + body + `</div><div style="padding:16px 28px;background:#fafaf7;color:#969c95;font-size:12px">此邮件由 {{site_name}} 自动发送，请勿直接回复。</div></div></body></html>`
	}
	return SMTPTemplateSettings{
		Register:       EmailTemplate{Subject: "{{site_name}} 注册验证码", HTML: wrapper("完成邮箱验证", "你好，{{email}}。请使用以下验证码完成注册。", `<div style="padding:18px 20px;background:#f3f1df;border-radius:7px;font-size:30px;font-weight:700;letter-spacing:6px;text-align:center">{{code}}</div><p style="margin:18px 0 0;color:#687068;font-size:13px">验证码将在 {{expire_minutes}} 分钟后失效。</p>`)},
		Reset:          EmailTemplate{Subject: "{{site_name}} 密码重置验证码", HTML: wrapper("重置账户密码", "我们收到了 {{email}} 的密码重置请求。", `<div style="padding:18px 20px;background:#f3f1df;border-radius:7px;font-size:30px;font-weight:700;letter-spacing:6px;text-align:center">{{code}}</div><p style="margin:18px 0 0;color:#687068;font-size:13px">验证码将在 {{expire_minutes}} 分钟后失效。如非本人操作，请忽略此邮件。</p>`)},
		Welcome:        EmailTemplate{Subject: "欢迎加入 {{site_name}}", HTML: wrapper("欢迎加入 {{site_name}}", "你好，{{username}}。你的创作账户已经准备就绪。", `<p style="margin:0;color:#687068;line-height:1.8">登录后即可管理生成任务、作品记录、API Key 与账户额度。</p>`)},
		WelcomeEnabled: false,
	}
}

func NewSMTPService() *SMTPService {
	return &SMTPService{}
}

func (s *SMTPService) SendCode(ctx context.Context, cfg SMTPConfig, to, code, purpose string) error {
	_ = ctx
	if strings.TrimSpace(cfg.Host) == "" || cfg.Port <= 0 || strings.TrimSpace(cfg.FromAddr) == "" {
		return errors.New("SMTP 未配置")
	}
	tpl := cfg.Templates.Register
	if purpose == "reset" {
		tpl = cfg.Templates.Reset
	}
	vars := emailVariables(cfg, to, strings.Split(to, "@")[0], code)
	subject := renderEmailTemplate(tpl.Subject, vars, false)
	body := renderEmailTemplate(tpl.HTML, vars, true)
	msg := buildSMTPMessage(cfg.FromAddr, to, subject, body, "text/html")
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))

	if cfg.UseTLS || cfg.Port == 465 {
		return sendMailTLS(addr, cfg, to, msg)
	}
	return sendMailSTARTTLS(addr, cfg, to, msg)
}

func (s *SMTPService) SendWelcome(ctx context.Context, cfg SMTPConfig, to, username string) error {
	_ = ctx
	if !cfg.Templates.WelcomeEnabled {
		return nil
	}
	if strings.TrimSpace(cfg.Host) == "" || cfg.Port <= 0 || strings.TrimSpace(cfg.FromAddr) == "" {
		return errors.New("SMTP 未配置")
	}
	vars := emailVariables(cfg, to, username, "")
	subject := renderEmailTemplate(cfg.Templates.Welcome.Subject, vars, false)
	body := renderEmailTemplate(cfg.Templates.Welcome.HTML, vars, true)
	msg := buildSMTPMessage(cfg.FromAddr, to, subject, body, "text/html")
	addr := net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port))
	if cfg.UseTLS || cfg.Port == 465 {
		return sendMailTLS(addr, cfg, to, msg)
	}
	return sendMailSTARTTLS(addr, cfg, to, msg)
}

func emailVariables(cfg SMTPConfig, emailAddr, username, code string) map[string]string {
	siteName := strings.TrimSpace(cfg.SiteName)
	if siteName == "" {
		siteName = "Vivid"
	}
	expire := cfg.ExpireMinutes
	if expire <= 0 {
		expire = 10
	}
	return map[string]string{
		"site_name": siteName, "email": emailAddr, "username": username,
		"code": code, "expire_minutes": strconv.Itoa(expire),
	}
}

func renderEmailTemplate(input string, vars map[string]string, escapeHTML bool) string {
	result := input
	for key, value := range vars {
		if escapeHTML {
			value = html.EscapeString(value)
		}
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	if !escapeHTML {
		result = strings.ReplaceAll(strings.ReplaceAll(result, "\r", ""), "\n", " ")
	}
	return result
}

func buildSMTPMessage(from, to, subject, body, contentType string) []byte {
	if contentType == "" {
		contentType = "text/plain"
	}
	lines := []string{
		"From: " + from,
		"To: " + to,
		"Subject: " + mime.QEncoding.Encode("UTF-8", subject),
		"MIME-Version: 1.0",
		"Content-Type: " + contentType + "; charset=UTF-8",
		"",
		body,
	}
	return []byte(strings.Join(lines, "\r\n"))
}

func sendMailTLS(addr string, cfg SMTPConfig, to string, msg []byte) error {
	conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: cfg.Host})
	if err != nil {
		return err
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, cfg.Host)
	if err != nil {
		return err
	}
	defer client.Close()
	return doSMTP(client, cfg, to, msg)
}

func sendMailSTARTTLS(addr string, cfg SMTPConfig, to string, msg []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return err
	}
	defer client.Close()

	if ok, _ := client.Extension("STARTTLS"); ok {
		if err := client.StartTLS(&tls.Config{ServerName: cfg.Host}); err != nil {
			return err
		}
	}
	return doSMTP(client, cfg, to, msg)
}

func doSMTP(client *smtp.Client, cfg SMTPConfig, to string, msg []byte) error {
	if strings.TrimSpace(cfg.Username) != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return err
		}
	}
	if err := client.Mail(cfg.FromAddr); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		_ = w.Close()
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return client.Quit()
}
