**Design QA**

- Reference: `D:\桌面\新文件2.tsx` and `codex-clipboard-aeee63c0-a0ea-488d-9292-fb4dc78a2eb6.png`.
- Runtime: `http://127.0.0.1:2100/`.
- Desktop viewport: `1440x1000`.
- Mobile viewport: `390x844`.
- Public evidence: `E:\zhuceji\image2api-saas\frontend\docs\public-final.png`.
- Public mobile evidence: `E:\zhuceji\image2api-saas\frontend\docs\public-mobile-final.png`.
- Admin evidence: `E:\zhuceji\image2api-saas\frontend\docs\admin-overview-final.png`.
- Admin mobile evidence: `E:\zhuceji\image2api-saas\frontend\docs\admin-overview-mobile-final.png`.
- Brand settings evidence: `E:\zhuceji\image2api-saas\frontend\docs\admin-settings-updated.png`.
- Model catalog/editor evidence: `E:\zhuceji\image2api-saas\qa-screens\models-desktop.png`.
- Provider batch import evidence: `E:\zhuceji\image2api-saas\qa-screens\providers-desktop.png`.
- CDK management evidence: `E:\zhuceji\image2api-saas\qa-screens\cdks-desktop.png` and `E:\zhuceji\image2api-saas\qa-screens\cdks-mobile.png`.
- User redemption evidence: `E:\zhuceji\image2api-saas\qa-screens\billing-desktop.png` and `E:\zhuceji\image2api-saas\qa-screens\billing-mobile.png`.

**Comparison**

- The admin overview follows the selected reference hierarchy: compact shell, four KPI cards, a large neutral dual-series line chart, segmented time controls, a dense operations table, and a secondary health panel.
- Styling stays within the product constraints: neutral SaaS palette, restrained olive/yellow accents, 8px maximum card radius, no blue-purple or pink gradients, no cyberpunk treatment, and no decorative card wall.
- The public home no longer contains the dark “从一句描述开始” band. Its hero, selected work, model directory, and OpenAI-compatible section remain visually balanced.
- Desktop sidebar expands to `268px` and collapses to `84px`; the state persists locally. The same neutral menu icon is used in both states, with no directional arrows.
- Anonymous users see “登录” in the top action. Protected navigation opens the login/register modal and keeps the root page public.
- The user shell contains no visible admin navigation. Admin remains available only through `/admin/...` and authorization checks.
- Site name and Logo are loaded from the public site settings. Logo upload, immediate shell refresh, removal, title, description, and operations contact fields were exercised against the real backend.
- Admin metrics, hourly image/video trend, recent requests, billing outcome, and Provider health use live API responses. Empty databases render explicit empty states instead of fabricated values.
- Provider management supports one-credential-per-line import, standard row selection, single delete, bulk delete, invalid-account cleanup, quota refresh, and real image/video model testing.
- Model management consumes the backend catalog directly. Catalog capabilities are read-only while normal pricing, agent pricing, API alias, weight, enable state, and real generation testing are operator-controlled.
- CDK operations support batch generation, normal/marketing modes, search and filters, copy, single delete, multi-select delete, and numeric pagination. User billing supports direct redemption independently of online payment availability.

**Verification**

- Frontend production build passed.
- Go test suite passed.
- Docker runtime was updated with the final frontend build.
- Browser checks passed for public desktop, public mobile, collapsed sidebar, admin desktop, admin mobile, protected navigation, and branding upload/removal.
- Live API QA passed for Provider bulk deletion; catalog add/remove; model test routing; CDK batch generation, redemption, balance update, redemption-state recording, and bulk deletion.
- Browser checks passed for model editor, Provider batch import, CDK management, and user redemption at `1440x960` and `390x844`.
- No tested page had horizontal overflow or browser console errors.
- Source scan found no directional arrow icons or arrow characters in the application UI.
- Temporary QA admin and uploaded QA Logo were removed after verification.

- Adobe account identity/quota evidence: `E:\zhuceji\image2api-saas\qa-screens\providers-fixed.png`.
- Proxy management evidence: `E:\zhuceji\image2api-saas\qa-screens\settings-proxy.png`.
- Runtime verification: 22 Adobe accounts active, all 22 identities resolved, with real quota requests using the configured proxy.
- Catalog verification: 21 unique model IDs; the README-listed `flux-kontext-max`, `gemini-veo31`, and `grok-image-quality` entries are present.
- Proxy settings browser check passed with no console errors or horizontal overflow.

**Operations Completion**

- Admin-managed enabled models are the sole source for the user generation selector; browser QA confirmed `Adobe Firefly GPT Image` is visible in `/app/generate`.
- The admin top bar no longer shows the user credit pill or the model creation action on any admin route.
- Admin navigation now includes a tested `返回创作空间` action on desktop and mobile menus; the user workspace still exposes no admin entry.
- System settings now provide eight operational areas: site branding, registration security, SMTP, credit rewards, 去 AI 特征, platform announcement, file retention, and per-media proxy routing.
- Registration reward, daily check-in, invitation reward, announcement, deAI pricing, media/log retention, and default/image/video proxy settings passed idempotent GET/PUT verification without changing saved production values.
- Announcement publishing passed end-to-end QA with a temporary normal user: popup display, acknowledgement, server-side seen state, configuration restore, and user cleanup.
- Works management displays real generated images/videos with preview, download, single delete, selection, and bulk delete. Branding and homepage assets are excluded from both works listing and retention cleanup.
- Invitation records and the user reward center are live, with invitation links, first-success completion state, reward totals, and daily check-in controls.
- Directional icons were removed globally from selects, number steppers, pagers, scrollable tabs, upload/download/export actions, and generation submit actions.
- Desktop and mobile browser QA passed for settings, works, invites, billing, logs, generation, history, rewards, and announcement popup with no horizontal overflow or console errors.
- New evidence: `E:\zhuceji\image2api-saas\qa-screens\settings-operations-desktop.png`, `settings-operations-mobile.png`, `works-desktop.png`, `works-mobile.png`, `invites-desktop.png`, `generate-managed-model-desktop.png`, `rewards-desktop.png`, `rewards-mobile.png`, and `announcement-user-mobile.png`.
- Adobe runtime verification: 22 active accounts, all identities resolved, with quota requests routed through the configured working proxy.

**Media And Refinement Completion**

- Protected media authentication was fixed end to end: a valid SPA Bearer session restores the HttpOnly cookie, and the PHP web proxy now forwards `Set-Cookie` to the browser.
- Browser QA confirmed real pixels load in admin generation-log thumbnails and the works gallery when starting with localStorage only; failed and pending jobs use stable placeholders instead of broken image requests.
- Model testing uses a verified two-column parameter/result layout. Generation logs and user history use fixed thumbnail columns, compact prompts/models, localized timestamps, and modal media preview.
- Homepage content editing no longer exposes raw image addresses. The generated-image picker renders six columns on desktop, and the empty preview was measured at the intended `172px` height after resolving a CSS min-height conflict.
- Reference image upload was browser-tested with a real local PNG: the uploaded count, thumbnail, index, and remove control rendered correctly.
- CDK action controls were measured with `flex-wrap: nowrap`; the admin sidebar scrollbar is hidden while retaining scroll behavior; authenticated top bars no longer show `开始生成`.
- SMTP now persists independent HTML templates for registration verification, password reset, and optional welcome mail. Template rendering, placeholder substitution, backend GET/PUT persistence, and the isolated browser preview all passed.
- Announcement delivery now refreshes after publishing and on a periodic interval. Admin and user acknowledgement both use the server-side content-version seen state.
- Targeted evidence: `E:\zhuceji\image2api-saas\qa-screens\showcase-picker-desktop.png`, `model-test-layout.png`, `smtp-templates-desktop.png`, and `reference-upload.png`.
- Final frontend production build and `go test ./...` passed.

**Viewer, Concurrency, And Reliability Completion**

- Admin works, admin generation logs, and user history now share a cardless full-screen media viewer. Image zoom in/out, original-size reset, wheel zoom, drag, keyboard close, and download are available without previous/next arrow controls.
- Browser QA confirmed both admin viewers expose three zoom actions, change the image transform after zoom, and render zero visible modal-card containers.
- The frontend freeze during admin generation tests was traced to the single-threaded PHP development server. The production-like local web runtime now uses Nginx, so long upstream generation requests no longer block navigation, static assets, health checks, or unrelated APIs.
- A real Adobe generation completed with HTTP 200 while navigation to generation logs took 39 ms and a concurrent health request took 14 ms.
- DeAI post-processing now fails and refunds the request when transformation fails instead of charging for an unchanged result. Unit tests cover valid PNG output, crop dimensions, and invalid input.
- Media responses use unbuffered Nginx proxying; the 1.6 MB branding asset no longer produces proxy temporary-file warnings.
- Redis 7 maintenance-notification probing is explicitly disabled for the single-node deployment; startup logs no longer contain the unsupported-command warning.
- The local backend Docker build was repaired by allowing the required `api-linux` artifact through `.dockerignore`; Compose rebuild and container recreation passed.
- Final verification passed: frontend production build, `go test ./...`, `go vet ./...`, Nginx configuration test, Docker health check, route checks, browser viewer QA, warning/error log scans, and temporary QA-data cleanup.

**Hero Rotation And Batch Generation Completion**

- The user sidebar now gives billing/credits and check-in/invitations distinct Arco icons. Runtime DOM verification confirmed the rendered SVG paths differ.
- The public Hero automatically rotates all three configured showcase images through front, middle, and back positions every 4.5 seconds. The sequence was observed across `Minimal form`, `Quiet objects`, and `cat`; hover pauses rotation and reduced-motion users receive no animated transition.
- Image generation now offers a stable 1/2/3/4 segmented quantity control. Total estimated credits are multiplied and precision-normalized, with a client-side insufficient-balance guard before submission.
- Each selected output creates an independent request and idempotency key, preserving the existing per-task reserve, success charge, and failure refund behavior. User credits refresh after the batch settles.
- The right output area renders queued, running, successful, and failed states independently. Desktop uses exactly three thumbnail columns; 390px mobile uses two columns with no horizontal overflow.
- Browser QA with delayed intercepted responses confirmed four running tasks (`0 / 4`), four loaded successful thumbnails (`4 / 4`), zoomable result preview, and a partial-failure case with three successes plus one explicit refunded failure.
- Final checks passed: frontend type check and production build, Nginx config, HTTP health, desktop/mobile overflow, Hero sequencing, icon distinction, browser console errors (0), uncaught exceptions (0), and temporary QA-user cleanup.

**Batch Billing And Log-State Completion**

- The multi-image instant failures were traced to the unique `credit_transactions.event_id` index: concurrent reservations all wrote the empty string before their generation events existed. Reservations now persist an unbound event as SQL `NULL`, and event attachment accepts both normalized `NULL` and legacy empty values.
- Startup migration normalizes historical empty event IDs to `NULL`; runtime verification found zero remaining empty values.
- `CreditTransaction.EventID` is nullable in the Go model, so an early generation-log failure can read and refund an unbound reservation without a database scan error.
- A PostgreSQL integration test concurrently reserves four charges for one user, verifies four unbound transactions, binds four unique events, and exercises an additional refund before event attachment. The Docker test passed in 0.05 seconds and removed all temporary users and transactions.
- The 1/2/3/4 quantity selector uses a high-contrast ink active state. Runtime browser QA confirmed the selected value is visually distinct.
- Admin generation logs now use the same circular olive loading treatment as the user workspace. Failed rows show `失败 · 已退款` with a compact localized reason and retain the full provider/database error in the hover title.
- Runtime browser QA confirmed the 54px pending thumbnail contains a 32px animated circular loader, legacy duplicate-key failures display `并发扣费事务冲突`, and the log page has no horizontal overflow.
- Final verification passed: `go test ./...`, `go vet ./...`, frontend type check and production build, Docker backend/web rebuild, backend startup, PostgreSQL integration test, browser runtime inspection, and QA-data cleanup.

**Billing Ledger And Log Identity Completion**

- Credit ledger responses now expose an explicit snake_case JSON contract. The user billing table receives populated transaction ID, event ID, reason, amount, balance, status, and timestamp fields instead of mismatched Go field names.
- Ledger rendering distinguishes `reserved`, `captured`, and `refunded` as `预扣处理中`, `已确认扣费`, and `已退款`; debit/refund signs and aggregate captured/refunded totals use the same normalized fields.
- The duplicate redemption control inside the balance summary was removed. The single `兑换码充值` action remains beside `在线充值` in the page heading and opens the existing redemption modal.
- Admin generation logs now have a dedicated `调用用户` column showing the resolved username and user ID. The request cell remains focused on the prompt and event ID.
- Live API QA returned complete reserved/captured/refunded fixtures through `/admin/api/billing/ledger`. Browser QA confirmed one redemption button, all three ledger states, the dedicated log-user column, and no horizontal overflow.
- Final checks passed: `go test ./...`, `go vet ./...`, frontend production build, backend/web Docker rebuild, health checks, browser runtime inspection, and temporary user/session/ledger/log cleanup.

## Sanbao video studio and operations QA (2026-08-15)
- Added a dedicated customer video studio with a large playback stage, capability-driven controls, generated-image picker, local image/video/audio references, 1/2/4 task submission, live progress, failure reason, and refund state.
- Added separate admin video-model and Sanbao-account pages. Verified empty, loading, import, pricing, enablement, account scheduling, test, and delete control layouts.
- Added persistent upstream task state, RustFS transfer, restart recovery, round-robin account scheduling, and reserved/captured/refunded billing integration.
- Fixed the stale-job window for long video renders and normalized public showcase/logo image authorization.
- Browser QA passed at 1440x1000 and 390x844 with no horizontal overflow, page errors, failed requests, clipped controls, or overlapping text.
- Final checks passed: `go test ./...`, `go vet ./...`, frontend production build, backend/web Docker deployment, and PostgreSQL/Redis/RustFS health checks.

## Capability-driven conversational video studio QA (2026-08-15)
- Replaced the single playback stage and oversized parameter form with a chronological conversation flow and a compact integrated composer.
- A generation batch now renders every video at once in a responsive two-column/one-column grid. Each result exposes its own model, ratio, duration, resolution, charge, progress, and refund reason.
- Removed all customer-facing upstream branding and provider filtering. Model names and runtime errors are sanitized before display while internal operations pages retain provider diagnostics.
- Model ratios, resolutions, durations, quantity options, prompt length, required images, media counts, total media count, and upload sizes are driven by the selected model capability snapshot.
- Server validation now rejects unadvertised duration, ratio, resolution, concurrency, missing required images, excessive media, and oversized encoded uploads before charging.
- Added focused tests for exact duration membership, JSON capability quantity parsing, and per-model upload-size enforcement.
- Production checks passed: `go test ./...`, `go vet ./...`, frontend type check/build, offline backend image rebuild, web image rebuild, Docker deployment, and desktop/mobile browser QA without overflow or runtime errors.

## Video composer surface cleanup (2026-08-15)
- Removed the nested textarea border, background, focus ring, and shadow so the composer reads as one unified surface instead of a card inside a card.
- Replaced the combined material menu with direct image, generated-work, video, and audio controls. Each control renders only while the selected model advertises remaining capacity for that media type and total media capacity is available.
- Browser capability mocking confirmed image-only models expose only `图片 / 作品库`, while mixed-media models expose `图片 / 作品库 / 视频 / 音频` without overflow.
- Desktop and 390px mobile QA passed with an actual zero-width textarea border, no console errors, and no horizontal overflow.

final result: passed
