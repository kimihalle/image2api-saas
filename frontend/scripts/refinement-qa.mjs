import { chromium } from 'playwright-core'
import path from 'node:path'

const base = process.env.QA_BASE_URL || 'http://127.0.0.1:2100'
const token = process.env.QA_ADMIN_TOKEN || ''
const browserPath = process.env.QA_BROWSER_PATH || 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'
if (!token) throw new Error('QA_ADMIN_TOKEN is required')

const browser = await chromium.launch({ executablePath: browserPath, headless: true })
const context = await browser.newContext({ viewport: { width: 1440, height: 960 } })
await context.addInitScript((session) => localStorage.setItem('northstar_session', session), token)
const page = await context.newPage()
const errors = []
page.on('pageerror', (error) => errors.push(error.message))
page.on('console', (message) => { if (message.type() === 'error') errors.push(message.text()) })

async function open(route) {
  await page.goto(base + route, { waitUntil: 'networkidle' })
  await page.locator('main').waitFor({ state: 'visible' })
  if ((await page.getByRole('button', { name: '我知道了', exact: true }).count()) > 0) {
    await page.getByRole('button', { name: '我知道了', exact: true }).click()
  }
  if ((await page.getByRole('button', { name: '开始生成', exact: true }).count()) > 0) throw new Error(`${route} still shows 开始生成 in top bar`)
}

const report = {}
try {
  await open('/admin/logs')
  const restoredCookie = (await context.cookies()).some((item) => item.name === 'vivid_session' && item.httpOnly)
  const logImage = page.locator('.asset-thumb img').first()
  const logImageLoaded = (await logImage.count()) > 0 && await logImage.evaluate((img) => img.naturalWidth > 0)
  if (!restoredCookie || !logImageLoaded) throw new Error(`media session restoration failed: cookie=${restoredCookie}, image=${logImageLoaded}`)
  report.logs = { restoredCookie, logImageLoaded, pendingPlaceholders: await page.locator('.asset-thumb.pending').count() }

  await open('/admin/works')
  const workImage = page.locator('.gallery-item img').first()
  const workImageLoaded = (await workImage.count()) > 0 && await workImage.evaluate((img) => img.naturalWidth > 0)
  if (!workImageLoaded) throw new Error('works gallery thumbnail did not load')
  await page.locator('.media-button').first().click()
  await page.locator('.arco-modal .preview-media').waitFor({ state: 'visible' })
  report.works = { workImageLoaded, galleryItems: await page.locator('.gallery-item').count(), cards: await page.locator('.work-card').count() }
  await page.keyboard.press('Escape')

  await open('/admin/showcase')
  await page.getByRole('button', { name: '新增内容', exact: true }).first().click()
  await page.getByRole('button', { name: '选择已生成图片', exact: true }).click()
  await page.locator('.generated-grid').waitFor({ state: 'visible' })
  const pickerColumns = await page.locator('.generated-grid').evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(' ').length)
  if (pickerColumns !== 6) throw new Error(`generated picker expected 6 columns, got ${pickerColumns}`)
  report.showcase = { pickerColumns, preview: await page.locator('.editor-preview').evaluate((element) => ({ height: element.getBoundingClientRect().height, cssHeight: getComputedStyle(element).height, attributes: [...element.attributes].map((item) => item.name), className: element.className })) }
  await page.screenshot({ path: path.resolve('../qa-screens/showcase-picker-desktop.png'), fullPage: true })
  await page.keyboard.press('Escape')
  await page.keyboard.press('Escape')

  await open('/admin/models')
  await page.getByRole('button', { name: '调用测试', exact: true }).first().click()
  await page.locator('.test-layout').waitFor({ state: 'visible' })
  const modalColumns = await page.locator('.test-layout').evaluate((element) => getComputedStyle(element).gridTemplateColumns.split(' ').length)
  if (modalColumns !== 2) throw new Error(`model test modal expected 2 columns, got ${modalColumns}`)
  report.modelTest = { modalColumns, resultPanel: await page.locator('.result-panel').count() }
  await page.screenshot({ path: path.resolve('../qa-screens/model-test-layout.png'), fullPage: true })
  await page.keyboard.press('Escape')

  await open('/admin/settings')
  await page.getByText('SMTP 邮件', { exact: true }).first().click()
  await page.locator('.template-manager').waitFor({ state: 'visible' })
  const frame = page.frameLocator('iframe[title="邮件模板预览"]')
  await frame.locator('body').waitFor({ state: 'visible' })
  const previewText = await frame.locator('body').innerText()
  if (!previewText.includes('完成邮箱验证')) throw new Error('SMTP template preview did not render')
  report.smtp = { modes: await page.locator('.template-toolbar .arco-radio').count(), previewRendered: true }
  await page.screenshot({ path: path.resolve('../qa-screens/smtp-templates-desktop.png'), fullPage: true })

  await open('/app/generate')
  const fixture = path.resolve('../qa-screens/works-desktop.png')
  await page.locator('#reference-file').setInputFiles(fixture)
  await page.getByText(/已上传 1 \/ .* 张参考图/).waitFor({ state: 'visible' })
  const referenceLoaded = await page.locator('.reference-strip img').first().evaluate((img) => img.naturalWidth > 0)
  if (!referenceLoaded) throw new Error('reference upload thumbnail did not render')
  report.referenceUpload = { referenceLoaded, count: await page.locator('.reference-strip figure').count() }
  await page.screenshot({ path: path.resolve('../qa-screens/reference-upload.png'), fullPage: true })

  await open('/admin/cdks')
  const action = page.locator('.action-row').first()
  if ((await action.count()) > 0) {
    const style = await action.evaluate((element) => ({ flexWrap: getComputedStyle(element).flexWrap, height: element.getBoundingClientRect().height }))
    if (style.flexWrap !== 'nowrap') throw new Error('CDK actions still wrap')
    report.cdk = style
  } else report.cdk = { empty: true }

  if (errors.length) throw new Error(`browser errors: ${errors.join(' | ')}`)
  console.log(JSON.stringify({ ok: true, report }, null, 2))
} finally {
  await context.close()
  await browser.close()
}
