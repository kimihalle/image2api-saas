import { chromium } from 'playwright-core'

const base = process.env.QA_BASE_URL || 'http://127.0.0.1:2100'
const token = process.env.QA_SESSION
if (!token) throw new Error('QA_SESSION is required')

const browser = await chromium.launch({ channel: 'chrome', headless: true })
const errors = []

async function verify(name, viewport, submit = false) {
  const context = await browser.newContext({ viewport })
  const page = await context.newPage()
  page.on('console', (message) => {
    if (message.type() === 'error') errors.push(`${name}: ${message.text()}`)
  })
  page.on('pageerror', (error) => errors.push(`${name}: ${error.message}`))
  page.on('response', (response) => {
    if (response.status() >= 500) errors.push(`${name}: HTTP ${response.status()} ${response.url()}`)
  })

  await page.goto(`${base}/`, { waitUntil: 'networkidle' })
  await page.evaluate((value) => localStorage.setItem('northstar_session', value), token)
  await page.goto(`${base}/app/generate`, { waitUntil: 'networkidle' })
  await page.locator('.studio').waitFor({ state: 'visible' })
  const dismiss = page.getByRole('button', { name: '我知道了' })
  if (await dismiss.isVisible().catch(() => false)) {
    await dismiss.click()
    await page.waitForTimeout(400)
  }
  if (submit && process.env.QA_MODEL) {
    await page.locator('.composer .arco-select').first().click()
    await page.locator('.arco-select-option').filter({ hasText: process.env.QA_MODEL }).click()
    await page.locator('.quantity-control button').filter({ hasText: /^2$/ }).click()
    await page.locator('.submit-row button').click()
    await page.waitForFunction(() => document.querySelectorAll('.output-card').length === 2)
    await page.waitForFunction(() => document.querySelectorAll('.output-card.failed').length === 2, null, { timeout: 15_000 })
    await page.locator('.submit-row button:not([disabled])').waitFor({ state: 'visible', timeout: 5_000 })
  }
  const metrics = await page.evaluate(() => ({
    path: location.pathname,
    title: document.title,
    heading: document.querySelector('.section-heading h2')?.textContent?.trim(),
    hasComposer: Boolean(document.querySelector('.composer')),
    hasPreview: Boolean(document.querySelector('.preview')),
    outputCards: document.querySelectorAll('.output-card').length,
    failedCards: document.querySelectorAll('.output-card.failed').length,
    horizontalOverflow: document.documentElement.scrollWidth > window.innerWidth + 1,
  }))
  await page.screenshot({ path: `../qa-screens/${name}.png`, fullPage: true })
  if (metrics.path !== '/app/generate' || metrics.heading !== '图片生成') {
    throw new Error(`${name}: generation route did not render: ${JSON.stringify(metrics)}`)
  }
  if (!metrics.hasComposer || !metrics.hasPreview || metrics.horizontalOverflow) {
    throw new Error(`${name}: invalid layout: ${JSON.stringify(metrics)}`)
  }
  console.log(JSON.stringify({ name, ...metrics }))
  await context.close()
}

try {
  await verify('async-generation-desktop', { width: 1440, height: 960 }, true)
  await verify('async-generation-mobile', { width: 390, height: 844 })
  if (errors.length) throw new Error(errors.join(' | '))
} finally {
  await browser.close()
}
