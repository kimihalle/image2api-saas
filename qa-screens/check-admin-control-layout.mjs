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
await page.addInitScript((token) => localStorage.setItem('northstar_session', token), login.token)

async function dismissAnnouncement() {
  const button = page.getByRole('button', { name: '我知道了' })
  try {
    await button.waitFor({ state: 'visible', timeout: 1500 })
    await button.click()
  } catch {
    // No active announcement.
  }
}

await page.goto('http://127.0.0.1:2100/admin/packages', { waitUntil: 'networkidle' })
await dismissAnnouncement()
const packageRow = page.locator('.package-row').first()
await packageRow.waitFor({ state: 'visible' })
const switchBox = await packageRow.locator('.arco-switch').boundingBox()
const deleteBox = await packageRow.getByRole('button', { name: '删除套餐' }).boundingBox()
if (!switchBox || Math.round(switchBox.width) !== 28 || Math.round(switchBox.height) !== 16) {
  throw new Error(`unexpected package switch size: ${JSON.stringify(switchBox)}`)
}
if (!deleteBox || Math.round(deleteBox.width) !== 30 || Math.round(deleteBox.height) !== 30) {
  throw new Error(`unexpected package delete size: ${JSON.stringify(deleteBox)}`)
}
await page.screenshot({ path: 'qa-screens/packages-compact-switch.png', fullPage: true })

await page.goto('http://127.0.0.1:2100/admin/banned-words', { waitUntil: 'networkidle' })
await dismissAnnouncement()
const addControl = page.locator('.add-control')
await addControl.waitFor({ state: 'visible' })
const inputBox = await addControl.locator('.arco-input-wrapper').boundingBox()
const selectBox = await addControl.locator('.arco-select-view').boundingBox()
const addBox = await addControl.getByRole('button', { name: '添加' }).boundingBox()
const importBox = await page.getByRole('button', { name: '批量导入' }).boundingBox()
const boxes = [inputBox, selectBox, addBox]
if (boxes.some((box) => !box || Math.round(box.height) !== 34)) {
  throw new Error(`banned word add controls are not 34px high: ${JSON.stringify(boxes)}`)
}
if (Math.max(...boxes.map((box) => box.y)) - Math.min(...boxes.map((box) => box.y)) > 1) {
  throw new Error(`banned word add controls are vertically misaligned: ${JSON.stringify(boxes)}`)
}
if (!selectBox || !addBox || addBox.x - (selectBox.x + selectBox.width) < 8) {
  throw new Error(`add action does not leave enough space after category select: ${JSON.stringify({ selectBox, addBox })}`)
}
if (!addBox || !importBox || addBox.x + addBox.width > importBox.x) {
  throw new Error(`add and import actions overlap: ${JSON.stringify({ addBox, importBox })}`)
}
await page.screenshot({ path: 'qa-screens/banned-words-aligned-add.png', fullPage: true })

await page.setViewportSize({ width: 390, height: 844 })
await page.reload({ waitUntil: 'networkidle' })
await dismissAnnouncement()
const horizontalOverflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
if (horizontalOverflow) throw new Error('mobile banned-word page has horizontal overflow')

if (errors.length) throw new Error(errors.join(' | '))
console.log(JSON.stringify({ switchBox, deleteBox, inputBox, selectBox, addBox, importBox, horizontalOverflow, errors }, null, 2))
await browser.close()
