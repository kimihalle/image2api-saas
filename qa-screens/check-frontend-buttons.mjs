import { chromium } from '../frontend/node_modules/playwright-core/index.mjs'

const token = process.env.QA_TOKEN
if (!token) throw new Error('QA_TOKEN is required')

const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
})
const errors = []
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
page.on('pageerror', (error) => errors.push(`pageerror: ${error.message}`))
page.on('console', (message) => {
  if (message.type() === 'error') errors.push(`console: ${message.text()}`)
})
await page.addInitScript((sessionToken) => {
  localStorage.setItem('northstar_session', sessionToken)
  localStorage.setItem('image_generation_mode', 'studio')
}, token)

const routes = ['/', '/app/generate', '/app/history', '/app/api-keys', '/app/docs', '/app/billing', '/app/rewards', '/app/settings']
const report = []
for (const route of routes) {
  await page.goto(`http://127.0.0.1:2100${route}`, { waitUntil: 'networkidle' })
  const buttons = await page.locator('.content .arco-btn:not(.arco-btn-circle)').evaluateAll((elements) => elements.map((element) => {
    const style = getComputedStyle(element)
    return {
      text: element.textContent?.trim().replace(/\s+/g, ' ') || '',
      radius: Number.parseFloat(style.borderTopLeftRadius),
      height: Math.round(element.getBoundingClientRect().height),
      background: style.backgroundColor,
      clipped: element.scrollWidth > element.clientWidth + 1,
    }
  }))
  const invalid = buttons.filter((button) => button.radius < 14 || button.height < 28 || button.clipped)
  if (invalid.length) throw new Error(`${route} has inconsistent buttons: ${JSON.stringify(invalid)}`)
  report.push({ route, buttons: buttons.length })
}
await page.goto('http://127.0.0.1:2100/app/generate', { waitUntil: 'networkidle' })
await page.screenshot({ path: 'qa-screens/frontend-buttons-desktop.png', fullPage: true })
await page.goto('http://127.0.0.1:2100/app/billing', { waitUntil: 'networkidle' })
await page.screenshot({ path: 'qa-screens/frontend-buttons-billing.png', fullPage: true })
await page.goto('http://127.0.0.1:2100/app/api-keys', { waitUntil: 'networkidle' })
await page.screenshot({ path: 'qa-screens/frontend-buttons-api-keys.png', fullPage: true })

const guest = await browser.newPage({ viewport: { width: 390, height: 844 } })
await guest.goto('http://127.0.0.1:2100/', { waitUntil: 'networkidle' })
await guest.getByRole('button', { name: '登录', exact: true }).click()
await guest.locator('.auth-card').waitFor({ state: 'visible' })
const loginRadius = await guest.locator('.auth-card .submit').evaluate((element) => Number.parseFloat(getComputedStyle(element).borderTopLeftRadius))
if (loginRadius < 20) throw new Error(`login submit is not a capsule: ${loginRadius}`)
const horizontalOverflow = await guest.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
if (horizontalOverflow) throw new Error('mobile login page has horizontal overflow')
await guest.screenshot({ path: 'qa-screens/frontend-buttons-login-mobile.png', fullPage: true })

if (errors.length) throw new Error(errors.join(' | '))
console.log(JSON.stringify({ report, loginRadius, horizontalOverflow, errors }, null, 2))
await browser.close()
