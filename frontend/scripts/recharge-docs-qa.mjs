import { chromium } from 'playwright-core'

const token = process.env.QA_SESSION
if (!token) throw new Error('QA_SESSION is required')

const browser = await chromium.launch({
  headless: true,
  executablePath: 'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe',
})
const errors = []

async function createPage(viewport) {
  const context = await browser.newContext({ viewport })
  const page = await context.newPage()
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(`console: ${message.text()}`)
  })
  page.on('pageerror', (error) => errors.push(`page: ${error.message}`))
  await page.goto('http://127.0.0.1:2100/', { waitUntil: 'networkidle' })
  await page.evaluate((value) => localStorage.setItem('northstar_session', value), token)
  return { context, page }
}

async function verifyRecharge(name, viewport) {
  const { context, page } = await createPage(viewport)
  await page.goto('http://127.0.0.1:2100/app/billing', { waitUntil: 'networkidle' })
  const announcementConfirm = page.getByRole('button', { name: '我知道了' })
  if (await announcementConfirm.count()) {
    await announcementConfirm.click()
    await page.waitForTimeout(200)
  }
  const blockingModalClose = page.locator('.arco-modal-wrapper:visible .arco-modal-close-btn')
  if (await blockingModalClose.count()) {
    await blockingModalClose.last().click()
    await page.waitForTimeout(200)
  }
  await page.getByRole('button', { name: /在线充值/ }).first().click()
  await page.getByRole('button', { name: '自定义金额' }).click()
  const input = page.locator('.amount-editor input')
  await input.fill('26.5')
  await input.blur()
  await page.waitForTimeout(300)

  const dialog = page.locator('.recharge-dialog')
  const text = await dialog.innerText()
  const creditPreview = await page.locator('.credit-preview').innerText()
  const confirmVisible = await page.getByRole('button', { name: /确认支付/ }).isVisible()
  await page.screenshot({ path: `../qa-screens/${name}.png`, fullPage: true })
  console.log(JSON.stringify({
    name,
    hasCustomMode: text.includes('自定义充值'),
    hasMinimum: text.includes('最低充值 ¥10.00'),
    hasCalculatedCredits: creditPreview.includes('预计到账') && creditPreview.includes('265') && creditPreview.includes('额度'),
    confirmVisible,
    horizontalOverflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
  }))
  await context.close()
}

async function verifyDocs() {
  const { context, page } = await createPage({ width: 1440, height: 1000 })
  await page.goto('http://127.0.0.1:2100/app/docs', { waitUntil: 'networkidle' })
  await page.locator('.model-row').first().waitFor()
  const models = await page.locator('.model-row').evaluateAll((rows) => rows.map((row) => row.textContent || ''))
  await page.screenshot({ path: '../qa-screens/docs-dynamic-models.png', fullPage: true })
  console.log(JSON.stringify({
    name: 'docs-dynamic-models',
    modelCount: models.length,
    hasEnabledImageModel: models.some((value) => value.includes('firefly-gpt-image-2')),
    hasDisabledModel: models.some((value) => value.includes('sanbao:gpt-image2-4K')),
    horizontalOverflow: await page.evaluate(() => document.documentElement.scrollWidth > innerWidth),
  }))
  await context.close()
}

await verifyRecharge('recharge-custom-desktop', { width: 1440, height: 1000 })
await verifyRecharge('recharge-custom-mobile', { width: 390, height: 844 })
await verifyDocs()
await browser.close()

if (errors.length) {
  console.error(JSON.stringify({ errors }, null, 2))
  process.exitCode = 1
}
