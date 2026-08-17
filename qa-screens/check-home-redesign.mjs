import { chromium } from '../frontend/node_modules/playwright-core/index.mjs'

const browser = await chromium.launch({
  headless: true,
  executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH,
})
const errors = []

for (const viewport of [{ name: 'desktop', width: 1440, height: 1000 }, { name: 'mobile', width: 390, height: 844 }]) {
  const page = await browser.newPage({ viewport })
  page.on('pageerror', (error) => errors.push(`${viewport.name}: ${error.message}`))
  page.on('console', (message) => { if (message.type() === 'error') errors.push(`${viewport.name}: ${message.text()}`) })
  await page.goto('http://127.0.0.1:2100/', { waitUntil: 'networkidle' })
  const close = page.locator('.arco-modal-container:visible .arco-modal-close-btn')
  if (await close.count()) await close.first().click()
  await page.getByRole('heading', { name: '精选作品', exact: true }).waitFor()
  const works = page.locator('.work-grid .work-card')
  const summaries = page.locator('.model-summary article')
  if (await works.count() !== 3) throw new Error(`${viewport.name}: expected 3 selected works`)
  if (await summaries.count() !== 2) throw new Error(`${viewport.name}: expected 2 model summaries`)
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth > document.documentElement.clientWidth)
  if (overflow) throw new Error(`${viewport.name}: horizontal overflow`)
  await page.screenshot({ path: `qa-screens/home-redesign-${viewport.name}.png`, fullPage: true })
  await page.close()
}

if (errors.length) throw new Error(errors.join(' | '))
console.log(JSON.stringify({ ok: true, errors }, null, 2))
await browser.close()
