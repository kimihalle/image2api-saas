import { chromium } from '../frontend/node_modules/playwright-core/index.mjs'

const email = process.env.QA_EMAIL
const password = process.env.QA_PASSWORD
if (!email || !password) throw new Error('QA_EMAIL and QA_PASSWORD are required')

const loginResponse = await fetch('http://127.0.0.1:2100/admin/api/auth/login', {
  method: 'POST',
  headers: { 'Content-Type': 'application/json' },
  body: JSON.stringify({ identifier: email, password }),
})
const login = await loginResponse.json()
if (!loginResponse.ok || !login.token) throw new Error(`QA login failed: ${JSON.stringify(login)}`)

const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
})
const page = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
const errors = []
page.on('pageerror', (error) => errors.push(`pageerror: ${error.message}`))
page.on('console', (message) => {
  if (message.type() === 'error') errors.push(`console: ${message.text()}`)
})
await page.addInitScript((token) => {
  localStorage.setItem('northstar_session', token)
  localStorage.setItem('image_generation_mode', 'studio')
}, login.token)

async function dismissAnnouncement() {
  const button = page.getByRole('button', { name: '我知道了' })
  try {
    await button.waitFor({ state: 'visible', timeout: 1500 })
    await button.click()
  } catch {
    // There is no active announcement or it has already been acknowledged.
  }
}

await page.goto('http://127.0.0.1:2100/admin/works', { waitUntil: 'networkidle' })
await dismissAnnouncement()
await page.locator('.works-page').waitFor({ state: 'visible' })
const items = page.locator('.gallery-item')
const itemCount = await items.count()
if (itemCount > 20) throw new Error(`works page rendered ${itemCount} items; expected at most 20`)
await page.getByText(/每页 20 个/).waitFor({ state: 'visible' })
if (await page.getByRole('button', { name: '下载作品' }).count() !== itemCount) {
  throw new Error('not every work has a download action')
}
const pickerBackground = itemCount
  ? await page.locator('.picker').first().evaluate((element) => getComputedStyle(element).backgroundColor)
  : 'rgba(0, 0, 0, 0)'
if (!pickerBackground.includes('0)') && pickerBackground !== 'transparent') {
  throw new Error(`work picker is not transparent: ${pickerBackground}`)
}
const operationLabels = await page.locator('.sidebar .nav-group').first().locator('.nav-list button').allTextContents()
const expectedOrder = ['运营概览', '生成日志', '作品管理', '用户与权限', '邀请记录', '订单与账本', '充值套餐', '兑换码管理', '首页内容', '灵感模板', '违禁词管理']
if (operationLabels.join('|') !== expectedOrder.join('|')) {
  throw new Error(`unexpected operation menu order: ${operationLabels.join('|')}`)
}
const adminButtons = await page.locator('.content .arco-btn:not(.arco-btn-circle)').evaluateAll((elements) => elements.map((element) => ({
  text: element.textContent?.trim().replace(/\s+/g, ' ') || '',
  radius: Number.parseFloat(getComputedStyle(element).borderTopLeftRadius),
  height: Math.round(element.getBoundingClientRect().height),
})))
const invalidAdminButtons = adminButtons.filter((button) => button.radius < 14 || button.height < 28)
if (invalidAdminButtons.length) throw new Error(`inconsistent admin buttons: ${JSON.stringify(invalidAdminButtons)}`)
if (itemCount) {
  await items.first().hover()
  const downloadButton = page.getByRole('button', { name: '下载作品' }).first()
  await downloadButton.waitFor({ state: 'visible' })
  const downloadEvent = page.waitForEvent('download')
  await downloadButton.click()
  const download = await downloadEvent
  await download.cancel()
  if (await page.locator('[class*="arco-image-preview"]:visible').count()) {
    throw new Error('download action also opened the media preview')
  }
  await items.first().hover()
}
await page.screenshot({ path: 'qa-screens/works-actions-pagination.png', fullPage: true })

await page.goto('http://127.0.0.1:2100/app/generate', { waitUntil: 'networkidle' })
await dismissAnnouncement()
const prompt = page.locator('.studio-prompt-input textarea')
await prompt.waitFor({ state: 'visible' })
const promptStyle = await prompt.evaluate((element) => ({
  resize: getComputedStyle(element).resize,
  placeholder: element.getAttribute('placeholder'),
}))
if (promptStyle.resize !== 'vertical') throw new Error(`prompt resize is ${promptStyle.resize}`)
if (!promptStyle.placeholder?.includes('双击')) throw new Error('prompt placeholder does not explain double-click editing')
await prompt.dblclick()
await page.locator('.prompt-editor-modal').waitFor({ state: 'visible' })
await page.waitForTimeout(250)
const modalPrompt = page.locator('.prompt-editor-modal textarea')
const sharedValue = '一段用于验证大输入框同步的完整生成描述'
await modalPrompt.fill(sharedValue)
await page.getByRole('button', { name: '完成编辑' }).click()
if (await prompt.inputValue() !== sharedValue) throw new Error('expanded prompt editor did not sync its content')
await prompt.dblclick()
await page.locator('.prompt-editor-modal').waitFor({ state: 'visible' })
await page.waitForTimeout(250)
await page.screenshot({ path: 'qa-screens/prompt-editor-desktop.png', fullPage: true })
await page.getByRole('button', { name: '完成编辑' }).click()

await page.setViewportSize({ width: 390, height: 844 })
await page.reload({ waitUntil: 'networkidle' })
await dismissAnnouncement()
await page.locator('.studio-prompt-input textarea').dblclick()
await page.locator('.prompt-editor-modal').waitFor({ state: 'visible' })
await page.waitForTimeout(250)
const horizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
if (horizontalOverflow) throw new Error('mobile prompt editor has horizontal overflow')
await page.screenshot({ path: 'qa-screens/prompt-editor-mobile.png', fullPage: true })

if (errors.length) throw new Error(errors.join(' | '))
console.log(JSON.stringify({ itemCount, pickerBackground, operationLabels, adminButtons, promptStyle, horizontalOverflow, errors }, null, 2))
await browser.close()
