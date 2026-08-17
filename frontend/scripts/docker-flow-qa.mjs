import { chromium } from 'playwright-core'

const base = process.env.QA_BASE_URL || 'http://127.0.0.1:2100'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const page = await browser.newPage({ viewport: { width: 1440, height: 960 } })
const errors = []
page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()) })
page.on('pageerror', (error) => errors.push(error.message))

try {
  await page.goto(`${base}/`, { waitUntil: 'networkidle' })
  await page.waitForURL(new RegExp(`${base.replace(/[.*+?^${}()|[\]\\]/g, '\\$&')}/?$`))
  await page.locator('.public-hero').waitFor({ state: 'visible' })

  await page.goto(`${base}/admin/overview`, { waitUntil: 'networkidle' })
  await page.waitForURL(/\/login(?:\?|$)/)

  await page.getByRole('button', { name: '还没有账户？创建账户' }).click()
  const suffix = Date.now().toString().slice(-8)
  const email = `qa-${suffix}@northstar.local`
  const username = `qa${suffix}`
  const password = 'Northstar#2026'
  const fields = page.locator('.arco-form-item input')
  await fields.nth(0).fill(email)
  await fields.nth(1).fill(username)
  await fields.nth(2).fill(password)
  await fields.nth(3).fill(password)
  const [registerResponse] = await Promise.all([
    page.waitForResponse((response) => response.url().includes('/admin/api/auth/register'), { timeout: 15_000 }),
    page.getByRole('button', { name: /创建账户/ }).click(),
  ])
  if (!registerResponse.ok()) throw new Error(`Registration failed (${registerResponse.status()}): ${await registerResponse.text()}`)
  await page.waitForURL(/\/(?:app|admin)(?:\/overview)?$/, { timeout: 15_000 })
  if (page.url().includes('/admin/')) await page.goto(`${base}/app/overview`, { waitUntil: 'networkidle' })
  await page.locator('.dashboard-page').waitFor({ state: 'visible' })

  const userState = await page.evaluate(async () => {
    const token = localStorage.getItem('northstar_session') || ''
    const response = await fetch('/admin/api/auth/me', { headers: { Authorization: `Bearer ${token}` } })
    return response.json()
  })
  const userText = await page.locator('body').innerText()
  if (userText.includes('管理后台') || userText.includes('运营后台')) throw new Error('User frontend exposes an admin entry')

  await page.goto(`${base}/admin/overview`, { waitUntil: 'networkidle' })
  if (userState.user?.role === 'admin') {
    await page.waitForURL(/\/admin\/overview$/)
    await page.getByText('运营概览', { exact: true }).first().waitFor()
  } else {
    await page.waitForURL(/\/app(?:\/overview)?$/)
  }

  let adminVerified = false
  if (process.env.QA_ADMIN_IDENTIFIER && process.env.QA_ADMIN_PASSWORD) {
    await page.evaluate(() => localStorage.clear())
    await page.goto(`${base}/admin/overview`, { waitUntil: 'networkidle' })
    await page.locator('.arco-form-item input').nth(0).fill(process.env.QA_ADMIN_IDENTIFIER)
    await page.locator('.arco-form-item input').nth(1).fill(process.env.QA_ADMIN_PASSWORD)
    await page.getByRole('button', { name: '登录', exact: true }).click()
    await page.waitForURL(/\/admin\/overview$/, { timeout: 15_000 })
    await page.getByText('运营概览', { exact: true }).first().waitFor()
    adminVerified = true
  }

  if (errors.length) throw new Error(`Browser errors: ${errors.join(' | ')}`)
  console.log(JSON.stringify({
    ok: true,
    registered: email,
    role: userState.user?.role,
    adminPath: page.url(),
    adminVerified,
    frontendAdminEntryHidden: true,
    errors,
  }, null, 2))
} finally {
  await browser.close()
}
