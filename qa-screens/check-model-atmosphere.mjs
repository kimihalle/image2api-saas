import { chromium } from '../frontend/node_modules/playwright-core/index.mjs'

const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
})
const errors = []

async function inspect(viewport, screenshot) {
  const page = await browser.newPage({ viewport })
  page.on('pageerror', (error) => errors.push(`pageerror: ${error.message}`))
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(`console: ${message.text()}`)
  })
  await page.goto('http://127.0.0.1:2100/', { waitUntil: 'networkidle' })
  const notice = page.locator('.arco-modal-container:visible .arco-modal-close-btn')
  if (await notice.count()) await notice.first().click()
  await page.locator('.model-summary').scrollIntoViewIfNeeded()
  await page.waitForTimeout(500)
  const firstFrame = await page.locator('.model-summary').screenshot()
  const canvases = await page.locator('.model-atmosphere').evaluateAll((items) => items.map((canvas) => {
    const context = canvas.getContext('webgl')
    return {
      width: canvas.width,
      height: canvas.height,
      drawingWidth: context?.drawingBufferWidth || 0,
      version: context?.getParameter(context.VERSION) || '',
      error: context?.getError() || 0,
    }
  }))
  await page.waitForTimeout(500)
  const secondFrame = await page.locator('.model-summary').screenshot()
  if (canvases.length !== 2) throw new Error(`expected two atmosphere canvases, got ${canvases.length}`)
  canvases.forEach((value, index) => {
    if (value.width < 300 || value.height < 180 || !value.drawingWidth || value.error) throw new Error(`canvas ${index} is invalid: ${JSON.stringify(value)}`)
  })
  if (firstFrame.equals(secondFrame)) throw new Error('smoke frames did not animate')
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
  if (overflow) throw new Error(`horizontal overflow at ${viewport.width}px`)
  await page.screenshot({ path: screenshot, fullPage: true })
  await page.close()
  return { viewport, canvases, animated: true, overflow }
}

const desktop = await inspect({ width: 1440, height: 1000 }, 'qa-screens/model-atmosphere-desktop.png')
const mobile = await inspect({ width: 390, height: 844 }, 'qa-screens/model-atmosphere-mobile.png')
if (errors.length) throw new Error(errors.join(' | '))
console.log(JSON.stringify({ desktop, mobile }, null, 2))
await browser.close()
