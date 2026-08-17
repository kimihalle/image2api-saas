import { chromium } from 'playwright-core'

const base = 'http://127.0.0.1:2100'
const browser = await chromium.launch({ headless: true, channel: 'msedge' })
const errors = []
const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, deviceScaleFactor: 1 })
const page = await context.newPage()
page.on('console', (message) => { if (message.type() === 'error') errors.push(`console: ${message.text()}`) })
page.on('pageerror', (error) => errors.push(`page: ${error.message}`))

await page.goto(`${base}/inspiration`, { waitUntil: 'networkidle' })
await page.locator('.template-item').first().waitFor()
while (await page.getByRole('button', { name: '加载更多' }).count()) {
  await page.getByRole('button', { name: '加载更多' }).scrollIntoViewIfNeeded()
  await page.getByRole('button', { name: '加载更多' }).click()
  await page.waitForTimeout(250)
}
await page.evaluate(async () => {
  for (let y = 0; y < document.documentElement.scrollHeight; y += 700) {
    window.scrollTo(0, y)
    await new Promise((resolve) => setTimeout(resolve, 30))
  }
  window.scrollTo(0, 0)
})
await page.waitForTimeout(800)

const result = await page.evaluate(() => {
  const cards = [...document.querySelectorAll('.template-item')]
  const images = cards.map((card) => card.querySelector('img')).filter(Boolean)
  return {
    totalLabel: document.querySelector('.library-stat strong')?.textContent || '',
    cards: cards.length,
    uniqueImages: new Set(images.map((image) => image.currentSrc || image.src)).size,
    brokenImages: images.filter((image) => !image.complete || image.naturalWidth === 0).length,
    categoryButtons: document.querySelectorAll('.category-pills button').length,
    overflow: document.documentElement.scrollWidth - window.innerWidth,
  }
})
if (result.cards !== 519) throw new Error(`expected 519 public templates, got ${result.cards}`)
if (result.uniqueImages !== result.cards) throw new Error(`expected unique cover per template, got ${result.uniqueImages}/${result.cards}`)
if (result.brokenImages) throw new Error(`${result.brokenImages} catalog images failed to load`)
if (result.overflow > 1) throw new Error(`desktop horizontal overflow: ${result.overflow}px`)
await page.screenshot({ path: '../qa-screens/inspiration-catalog.png' })

await context.close()

const narrowContext = await browser.newContext({ viewport: { width: 900, height: 760 }, deviceScaleFactor: 1 })
const narrowPage = await narrowContext.newPage()
await narrowPage.goto(`${base}/inspiration`, { waitUntil: 'networkidle' })
const categoryNav = narrowPage.locator('.category-pills')
await categoryNav.waitFor()
const beforeScroll = await categoryNav.evaluate((element) => ({ left: element.scrollLeft, max: element.scrollWidth - element.clientWidth }))
await categoryNav.hover()
await narrowPage.mouse.wheel(0, 420)
await narrowPage.waitForTimeout(250)
const afterScroll = await categoryNav.evaluate((element) => element.scrollLeft)
if (beforeScroll.max > 0 && afterScroll <= beforeScroll.left) throw new Error('category wheel scrolling did not move the tab strip')
result.categoryScroll = { max: beforeScroll.max, after: afterScroll }
await narrowPage.screenshot({ path: '../qa-screens/inspiration-category-scroll.png' })
await narrowContext.close()

await browser.close()
if (errors.length) throw new Error(errors.join('\n'))
console.log(JSON.stringify(result))
