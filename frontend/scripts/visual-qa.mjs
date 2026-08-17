import { chromium } from 'playwright-core'
import { mkdir } from 'node:fs/promises'
import path from 'node:path'

const base = process.env.QA_BASE_URL || 'http://127.0.0.1:2100'
const output = path.resolve('../docs/screenshots')
const identifier = process.env.QA_IDENTIFIER || ''
const password = process.env.QA_PASSWORD || ''
await mkdir(output, { recursive: true })

const browser = await chromium.launch({ channel: 'chrome', headless: true })
const report = []
const arrowPattern = /[→←↓↑➜]/

async function capture(context, item) {
  const page = await context.newPage()
  await page.setViewportSize({ width: item.width, height: item.height })
  const errors = []
  page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()) })
  page.on('pageerror', (error) => errors.push(error.message))
  await page.goto(base + item.path, { waitUntil: 'networkidle' })
  const finalPath = new URL(page.url()).pathname
  const bodyText = await page.locator('body').innerText()
  await page.screenshot({ path: path.join(output, `${item.name}.png`), fullPage: false })
  if (item.fullPage) await page.screenshot({ path: path.join(output, `${item.name}-full.png`), fullPage: true })
  const layout = await page.evaluate(() => ({
    title: document.title,
    scrollWidth: document.documentElement.scrollWidth,
    clientWidth: document.documentElement.clientWidth,
    bodyHeight: document.body.scrollHeight,
  }))
  report.push({
    ...item,
    finalPath,
    ...layout,
    horizontalOverflow: layout.scrollWidth > layout.clientWidth + 1,
    hasDirectionalArrow: arrowPattern.test(bodyText),
    errors,
  })
  await page.close()
}

try {
  const publicContext = await browser.newContext()
  const publicCases = [
    { name: 'public-home-desktop', path: '/', width: 2416, height: 1144, fullPage: true },
    { name: 'public-home-mobile', path: '/', width: 390, height: 844, fullPage: true },
    { name: 'login-desktop', path: '/login', width: 1440, height: 960 },
    { name: 'login-mobile', path: '/login', width: 390, height: 844 },
    { name: 'anonymous-app-redirect', path: '/app/overview', width: 1440, height: 960 },
  ]
  for (const item of publicCases) await capture(publicContext, item)
  await publicContext.close()

  if (identifier && password) {
    const authContext = await browser.newContext()
    const loginPage = await authContext.newPage()
    await loginPage.goto(`${base}/login`, { waitUntil: 'networkidle' })
    const fields = loginPage.locator('.arco-form-item input')
    await fields.nth(0).fill(identifier)
    await fields.nth(1).fill(password)
    await loginPage.getByRole('button', { name: '登录', exact: true }).click()
    await loginPage.waitForURL(/\/app(?:\/overview)?$/, { timeout: 15_000 })
    const userText = await loginPage.locator('body').innerText()
    if (userText.includes('管理后台') || userText.includes('运营后台')) {
      throw new Error('User workspace exposes an admin entry')
    }
    await loginPage.close()

    const authCases = [
      { name: 'dashboard-desktop', path: '/app/overview', width: 1440, height: 960 },
      { name: 'dashboard-mobile', path: '/app/overview', width: 390, height: 844 },
      { name: 'admin-showcase-desktop', path: '/admin/showcase', width: 1440, height: 960 },
      { name: 'admin-overview-desktop', path: '/admin/overview', width: 1440, height: 960 },
    ]
    for (const item of authCases) await capture(authContext, item)
    await authContext.close()
  }

  const failures = report.filter((item) => item.horizontalOverflow || item.hasDirectionalArrow || item.errors.length)
  if (failures.length) throw new Error(`Visual QA failures: ${JSON.stringify(failures)}`)
  const redirect = report.find((item) => item.name === 'anonymous-app-redirect')
  if (redirect?.finalPath !== '/login') throw new Error(`Anonymous app route did not redirect to login: ${redirect?.finalPath}`)
  console.log(JSON.stringify({ ok: true, authenticatedCases: Boolean(identifier && password), report }, null, 2))
} finally {
  await browser.close()
}
