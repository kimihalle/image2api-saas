import { chromium } from 'playwright-core'

const base = 'http://127.0.0.1:2100'
const userToken = 'TESTUSERPROMPTSESSION000000000000000000000000000001'
const adminToken = 'TESTADMINPROMPTSESSION0000000000000000000000000001'
const browser = await chromium.launch({ headless: true, channel: 'msedge' })
const errors = []

async function contextWithToken(viewport, token = '') {
  const context = await browser.newContext({ viewport, deviceScaleFactor: 1 })
  if (token) await context.addInitScript(([key, value]) => localStorage.setItem(key, value), ['northstar_session', token])
  return context
}

const desktop = await contextWithToken({ width: 1440, height: 1000 }, userToken)
const page = await desktop.newPage()
page.on('console', (message) => { if (message.type() === 'error') errors.push(`console: ${message.text()}`) })
page.on('pageerror', (error) => errors.push(`page: ${error.message}`))
await page.goto(`${base}/inspiration`, { waitUntil: 'networkidle' })
await page.locator('.template-item').first().waitFor()
const templateCount = await page.locator('.template-item').count()
if (templateCount < 10) throw new Error(`expected template grid, got ${templateCount}`)
await page.screenshot({ path: '../qa-screens/inspiration-desktop.png', fullPage: true })

await page.getByText('极简产品主图', { exact: true }).click()
await page.locator('.prompt-detail-modal').waitFor()
await page.getByPlaceholder('例如：银色无线耳机').fill('白色便携咖啡机')
await page.screenshot({ path: '../qa-screens/inspiration-detail.png' })
await page.getByRole('button', { name: '使用此模板' }).click()
await page.waitForURL('**/app/generate?template=*')
await page.locator('textarea').first().waitFor()
const prompt = await page.locator('textarea').first().inputValue()
if (!prompt.includes('白色便携咖啡机')) throw new Error('template prompt was not applied to image studio')
await page.screenshot({ path: '../qa-screens/inspiration-applied.png' })
await desktop.close()

const mobile = await contextWithToken({ width: 390, height: 844 })
const mobilePage = await mobile.newPage()
mobilePage.on('console', (message) => { if (message.type() === 'error') errors.push(`mobile console: ${message.text()}`) })
mobilePage.on('pageerror', (error) => errors.push(`mobile page: ${error.message}`))
await mobilePage.goto(`${base}/inspiration`, { waitUntil: 'networkidle' })
await mobilePage.locator('.template-item').first().waitFor()
const overflow = await mobilePage.evaluate(() => document.documentElement.scrollWidth - window.innerWidth)
if (overflow > 1) throw new Error(`mobile horizontal overflow: ${overflow}px`)
await mobilePage.screenshot({ path: '../qa-screens/inspiration-mobile.png', fullPage: true })
await mobile.close()

const admin = await contextWithToken({ width: 1440, height: 1000 }, adminToken)
const adminPage = await admin.newPage()
adminPage.on('console', (message) => { if (message.type() === 'error') errors.push(`admin console: ${message.text()}`) })
adminPage.on('pageerror', (error) => errors.push(`admin page: ${error.message}`))
await adminPage.goto(`${base}/admin/prompts`, { waitUntil: 'networkidle' })
await adminPage.locator('.arco-table-tr:not(.arco-table-tr-empty)').first().waitFor()
const adminRows = await adminPage.locator('.arco-table-tr:not(.arco-table-tr-empty)').count()
if (adminRows < 1) throw new Error('admin prompt table is empty')
await adminPage.screenshot({ path: '../qa-screens/prompt-admin-desktop.png', fullPage: true })
await adminPage.getByText('内容同步', { exact: true }).click()
await adminPage.locator('.sync-panel').waitFor()
await adminPage.screenshot({ path: '../qa-screens/prompt-admin-import.png', fullPage: true })
await admin.close()

await browser.close()
if (errors.length) throw new Error(errors.join('\n'))
console.log(JSON.stringify({ templateCount, adminRows, promptApplied: true, mobileOverflow: overflow }))
