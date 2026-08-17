import { chromium } from '../frontend/node_modules/playwright-core/index.mjs'

const userToken = process.env.QA_TOKEN
const adminToken = process.env.QA_ADMIN_TOKEN
const executablePath = process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH
if (!userToken || !adminToken || !executablePath) throw new Error('QA_TOKEN, QA_ADMIN_TOKEN and PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH are required')

const browser = await chromium.launch({ headless: true, executablePath })
const errors = []
const report = {}

function watch(page, scope) {
  page.on('pageerror', (error) => errors.push(`${scope} pageerror: ${error.message}`))
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(`${scope} console: ${message.text()}`)
  })
}

async function dismissAnnouncement(page) {
  const button = page.getByRole('button', { name: '我知道了' })
  if (await button.count() && await button.first().isVisible()) await button.first().click()
  const close = page.locator('.arco-modal-container:visible .arco-modal-close-btn')
  if (await close.count()) await close.first().click()
}

const guest = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
watch(guest, 'guest')
await guest.goto('http://127.0.0.1:2100/', { waitUntil: 'networkidle' })
await dismissAnnouncement(guest)
await guest.locator('.model-summary article').first().waitFor()
const cards = await guest.locator('.model-summary article').allTextContents()
if (cards.length !== 2) throw new Error(`expected 2 model summary cards, got ${cards.length}`)
const managedModels = await guest.evaluate(async () => (await fetch('/admin/api/managed-models')).json())
const enabledModels = (managedModels.data || managedModels || []).filter((item) => item.enabled !== false)
const imageCount = enabledModels.filter((item) => item.type === 'image').length
const videoCount = enabledModels.filter((item) => item.type === 'video').length
if (!cards[0].includes('图片生成') || !cards[0].includes(`${imageCount}个模型`)) throw new Error(`unexpected image card: ${cards[0]}`)
if (!cards[1].includes('视频创作') || !cards[1].includes(`${videoCount}个模型`)) throw new Error(`unexpected video card: ${cards[1]}`)
const contacts = await guest.locator('.site-footer .footer-contact a').allTextContents()
for (const expected of ['vividairun@gmail.com', '1114639355', '1106849765', '购买兑换码']) {
  if (!contacts.some((value) => value.includes(expected))) throw new Error(`missing footer contact: ${expected}`)
}
if (await guest.locator('.workspace-switch, .mobile-workspace').count()) throw new Error('workspace card still exists on public layout')
await guest.screenshot({ path: 'qa-screens/public-home-commerce-desktop.png', fullPage: true })
report.home = { cards, contacts, imageCount, videoCount }

const user = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
watch(user, 'user')
await user.addInitScript((token) => localStorage.setItem('northstar_session', token), userToken)
await user.goto('http://127.0.0.1:2100/app/billing', { waitUntil: 'networkidle' })
await dismissAnnouncement(user)
if (await user.locator('.workspace-switch, .mobile-workspace').count()) throw new Error('personal workspace card still exists')
await user.getByRole('button', { name: '兑换码充值' }).click()
const purchase = user.getByRole('button', { name: '购买兑换码' })
await purchase.waitFor()
await user.evaluate(() => {
  window.__qaOpenedURLs = []
  window.open = (url) => { window.__qaOpenedURLs.push(String(url)); return null }
})
await purchase.click()
const purchaseURL = await user.evaluate(() => window.__qaOpenedURLs[0] || '')
if (!purchaseURL.startsWith('https://pay.ldxp.cn/shop/chiyi')) throw new Error(`unexpected purchase URL: ${purchaseURL}`)
await user.screenshot({ path: 'qa-screens/cdk-purchase-dialog-desktop.png', fullPage: true })
report.billing = { purchaseURL }

const admin = await browser.newPage({ viewport: { width: 1440, height: 1000 } })
watch(admin, 'admin')
await admin.addInitScript((token) => localStorage.setItem('northstar_session', token), adminToken)
await admin.goto('http://127.0.0.1:2100/admin/settings', { waitUntil: 'networkidle' })
await dismissAnnouncement(admin)
if (await admin.locator('.workspace-switch, .mobile-workspace').count()) throw new Error('operations workspace card still exists')
await admin.getByText('兑换码购买地址', { exact: true }).waitFor()
await admin.getByText('客服 QQ 链接', { exact: true }).waitFor()
await admin.getByText('QQ 群链接', { exact: true }).waitFor()
await admin.screenshot({ path: 'qa-screens/admin-contact-purchase-settings.png', fullPage: true })
report.admin = { settingsVisible: true }

const mobile = await browser.newPage({ viewport: { width: 390, height: 844 } })
watch(mobile, 'mobile')
await mobile.goto('http://127.0.0.1:2100/', { waitUntil: 'networkidle' })
await dismissAnnouncement(mobile)
const overflow = await mobile.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
if (overflow) throw new Error('public home has horizontal overflow at 390px')
await mobile.screenshot({ path: 'qa-screens/public-home-commerce-mobile.png', fullPage: true })
report.mobile = { overflow }

if (errors.length) throw new Error(errors.join(' | '))
console.log(JSON.stringify(report, null, 2))
await browser.close()
