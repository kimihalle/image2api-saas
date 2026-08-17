import { chromium } from 'playwright-core'

const executablePath = 'C:/Program Files (x86)/Google/Chrome/Application/chrome.exe'
const token = process.env.QA_SESSION
if (!token) throw new Error('QA_SESSION is required')

const browser = await chromium.launch({ headless: true, executablePath })
const errors = []
async function verify(name, url, viewport) {
  const context = await browser.newContext({ viewport })
  const page = await context.newPage()
  page.on('console', msg => { if (msg.type() === 'error') errors.push(`${name}: ${msg.text()}`) })
  page.on('pageerror', error => errors.push(`${name}: ${error.message}`))
	page.on('response', response => { if (response.status() >= 400) errors.push(`${name}: HTTP ${response.status()} ${response.url()}`) })
  await page.goto('http://127.0.0.1:2100/', { waitUntil: 'networkidle' })
  await page.evaluate(value => localStorage.setItem('northstar_session', value), token)
  await page.goto(`http://127.0.0.1:2100${url}`, { waitUntil: 'networkidle' })
  await page.waitForTimeout(800)
  const metrics = await page.evaluate(() => ({
    title: document.title,
    path: location.pathname,
    width: innerWidth,
    scrollWidth: document.documentElement.scrollWidth,
    bodyText: document.body.innerText.slice(0, 500),
		images: Array.from(document.images).map(img => ({ src: img.src, alt: img.alt, visible: !!(img.offsetWidth || img.offsetHeight) })),
  }))
  await page.screenshot({ path: `../qa-screens/${name}.png`, fullPage: true })
  console.log(JSON.stringify({ name, ...metrics }))
  await context.close()
}

await verify('video-studio-desktop', '/app/video', { width: 1440, height: 1000 })
await verify('video-models-desktop', '/admin/video-models', { width: 1440, height: 1000 })
await verify('sanbao-accounts-desktop', '/admin/sanbao-accounts', { width: 1440, height: 1000 })
await verify('video-studio-mobile', '/app/video', { width: 390, height: 844 })
await browser.close()
if (errors.length) {
  console.error(JSON.stringify({ errors }, null, 2))
  process.exitCode = 1
}
