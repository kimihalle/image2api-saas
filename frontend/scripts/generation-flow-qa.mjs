import { chromium } from 'playwright-core'

const base = process.env.QA_BASE_URL || 'http://127.0.0.1:5174'
const browser = await chromium.launch({ channel: 'chrome', headless: true })
const page = await browser.newPage({ viewport: { width: 1440, height: 960 } })
const errors = []

page.on('console', (message) => {
  if (message.type() === 'error') errors.push(message.text())
})
page.on('pageerror', (error) => errors.push(error.message))

try {
  await page.goto(`${base}/app/generate`, { waitUntil: 'networkidle' })
  await page.locator('.submit-row button').click()
  await page.locator('.preview.hasResult img').waitFor({ state: 'visible', timeout: 15_000 })
  await page.locator('.arco-message-success').waitFor({ state: 'visible', timeout: 5_000 })
  await page.waitForFunction(() => {
    const image = document.querySelector('.preview.hasResult img')
    return image instanceof HTMLImageElement && image.complete && image.naturalWidth > 0
  }, { timeout: 15_000 })

  const result = await page.locator('.preview.hasResult').evaluate((preview) => {
    const image = preview.querySelector('img')
    const metadata = preview.querySelector('.result-meta')
    return {
      imageLoaded: image instanceof HTMLImageElement && image.complete && image.naturalWidth > 0,
      imageUrl: image?.getAttribute('src') || '',
      metadata: metadata?.textContent?.replace(/\s+/g, ' ').trim() || '',
    }
  })

  if (!result.imageLoaded) throw new Error('Generated result image did not load')
  if (!result.metadata) throw new Error('Generated result metadata is missing')
  if (errors.length) throw new Error(`Browser errors: ${errors.join(' | ')}`)

  console.log(JSON.stringify({ ok: true, ...result, errors }, null, 2))
} finally {
  await browser.close()
}
