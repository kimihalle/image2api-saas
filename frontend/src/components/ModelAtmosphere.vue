<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

type Tone = 'image' | 'video'

const props = defineProps<{ tone: Tone }>()
const canvas = ref<HTMLCanvasElement | null>(null)
let gl: WebGLRenderingContext | null = null
let program: WebGLProgram | null = null
let frame = 0
let visible = true
let lastFrame = 0
let startTime = 0
let resizeObserver: ResizeObserver | null = null
let intersectionObserver: IntersectionObserver | null = null
let reducedMotion: MediaQueryList | null = null
let resolutionUniform: WebGLUniformLocation | null = null
let timeUniform: WebGLUniformLocation | null = null
let toneUniform: WebGLUniformLocation | null = null

const vertexSource = `
attribute vec2 a_position;
void main() {
  gl_Position = vec4(a_position, 0.0, 1.0);
}
`

// Domain-warped fBm creates rolling density instead of decorative motion lines.
const fragmentSource = `
precision highp float;
uniform vec2 u_resolution;
uniform float u_time;
uniform float u_tone;

float hash(vec2 p) {
  p = fract(p * vec2(123.34, 456.21));
  p += dot(p, p + 45.32);
  return fract(p.x * p.y);
}

float noise(vec2 p) {
  vec2 i = floor(p);
  vec2 f = fract(p);
  f = f * f * (3.0 - 2.0 * f);
  float a = hash(i);
  float b = hash(i + vec2(1.0, 0.0));
  float c = hash(i + vec2(0.0, 1.0));
  float d = hash(i + vec2(1.0, 1.0));
  return mix(mix(a, b, f.x), mix(c, d, f.x), f.y);
}

float fbm(vec2 p) {
  float value = 0.0;
  float amplitude = 0.52;
  mat2 rotation = mat2(0.80, 0.60, -0.60, 0.80);
  for (int octave = 0; octave < 5; octave++) {
    value += amplitude * noise(p);
    p = rotation * p * 2.03 + vec2(17.17, 9.23);
    amplitude *= 0.50;
  }
  return value;
}

float smokeField(vec2 p, float time, float offset) {
  vec2 flow = vec2(time * 0.035, -time * 0.13);
  vec2 warp;
  warp.x = fbm(p * 1.12 + flow + vec2(offset, 1.7));
  warp.y = fbm(p * 1.08 - flow * 0.72 + vec2(4.8, offset));
  vec2 curled = p + (warp - 0.5) * 1.62;
  float body = fbm(curled * 1.38 + vec2(-time * 0.025, -time * 0.10));
  float folds = fbm(curled * 3.15 + vec2(time * 0.018, -time * 0.19));
  float filaments = 1.0 - abs(fbm(curled * 5.2 - vec2(0.0, time * 0.24)) * 2.0 - 1.0);
  return body * 0.68 + folds * 0.24 + filaments * 0.11;
}

float starLayer(vec2 coordinates, float scale, float seed, float time) {
  vec2 cell = coordinates * scale;
  vec2 identity = floor(cell);
  vec2 local = fract(cell) - 0.5;
  vec2 offset = vec2(hash(identity + seed), hash(identity + seed + 19.73)) - 0.5;
  float randomValue = hash(identity + seed * 7.31);
  float distanceToStar = length(local - offset * 0.68);
  float radius = 0.010 + pow(randomValue, 7.0) * 0.052;
  float core = 1.0 - smoothstep(radius, radius + 0.018, distanceToStar);
  float glow = exp(-distanceToStar * 16.0) * pow(randomValue, 4.0) * 0.22;
  float twinkle = 0.58 + 0.42 * sin(time * (0.45 + randomValue * 1.15) + randomValue * 31.0);
  return (core + glow) * twinkle;
}

float segmentDistance(vec2 point, vec2 start, vec2 end) {
  vec2 line = end - start;
  float amount = clamp(dot(point - start, line) / dot(line, line), 0.0, 1.0);
  return length(point - start - line * amount);
}

float starBurst(vec2 point, vec2 position, float size, float time) {
  vec2 delta = point - position;
  float distanceToCore = length(delta);
  float core = exp(-distanceToCore * 210.0 / size);
  float horizontal = exp(-abs(delta.y) * 680.0 / size) * exp(-abs(delta.x) * 24.0 / size);
  float vertical = exp(-abs(delta.x) * 680.0 / size) * exp(-abs(delta.y) * 24.0 / size);
  float pulse = 0.72 + 0.28 * sin(time * 0.72 + position.x * 17.0);
  return (core + (horizontal + vertical) * 0.34) * pulse;
}

void main() {
  vec2 uv = gl_FragCoord.xy / u_resolution.xy;
  vec2 p = (gl_FragCoord.xy * 2.0 - u_resolution.xy) / min(u_resolution.x, u_resolution.y);
  float time = u_time;

  float bend = 0.18 * sin(p.y * 1.55 + time * 0.12);
  float plumeDistance = abs(p.x - 0.58 - bend);
  float plume = 1.0 - smoothstep(0.22, 1.62, plumeDistance);
  plume *= smoothstep(-1.28, -0.30, p.y) * (1.0 - smoothstep(0.76, 1.54, p.y));

  float mainField = smokeField(p * vec2(0.86, 1.0), time, 0.0);
  float rearField = smokeField((p + vec2(-0.34, 0.12)) * 1.18, time * 0.82, 7.0);
  float density = smoothstep(0.42, 0.82, mainField) * plume;
  density += smoothstep(0.49, 0.84, rearField) * plume * 0.48;
  density = clamp(density, 0.0, 1.0);

  float inner = smoothstep(0.57, 0.88, mainField) * plume;
  float edgeSample = smokeField(p * vec2(0.86, 1.0) + vec2(0.018, -0.012), time, 0.0);
  float smokeEdge = clamp((mainField - edgeSample) * 7.0 + 0.5, 0.0, 1.0) * density;

  vec3 videoBase = vec3(0.025, 0.065, 0.052);
  vec3 videoLow = vec3(0.055, 0.25, 0.17);
  vec3 videoMid = vec3(0.24, 0.54, 0.37);
  vec3 videoHigh = vec3(0.72, 0.88, 0.69);
  vec3 smokeColor = mix(videoLow, videoMid, clamp(mainField * 1.18, 0.0, 1.0));
  smokeColor = mix(smokeColor, videoHigh, inner * 0.48 + smokeEdge * 0.22);
  vec3 videoColor = mix(videoBase, smokeColor, density * 0.90);
  videoColor += videoHigh * smokeEdge * 0.10;

  vec3 imageBase = vec3(0.026, 0.028, 0.024);
  vec3 warmGold = vec3(0.78, 0.62, 0.25);
  vec3 paleLight = vec3(1.0, 0.94, 0.74);
  float aspect = u_resolution.x / u_resolution.y;
  vec2 starCoordinates = vec2(uv.x * aspect, uv.y);
  float stars = 0.0;
  stars += starLayer(starCoordinates + vec2(time * 0.0025, 0.0), 15.0, 2.0, time) * 0.48;
  stars += starLayer(starCoordinates + vec2(time * 0.0050, 0.0), 25.0, 8.0, time) * 0.70;
  stars += starLayer(starCoordinates + vec2(time * 0.0090, 0.0), 39.0, 14.0, time) * 0.90;
  float fieldMask = smoothstep(0.30, 0.58, uv.x);

  vec2 pointA = vec2(0.59, 0.29 + sin(time * 0.05) * 0.006);
  vec2 pointB = vec2(0.72, 0.43 + cos(time * 0.04) * 0.006);
  vec2 pointC = vec2(0.84, 0.31 + sin(time * 0.045) * 0.005);
  vec2 pointD = vec2(0.91, 0.59 + cos(time * 0.052) * 0.006);
  vec2 pointE = vec2(0.75, 0.70 + sin(time * 0.038) * 0.006);
  float constellation = 0.0;
  constellation += exp(-segmentDistance(uv, pointA, pointB) * 520.0);
  constellation += exp(-segmentDistance(uv, pointB, pointC) * 520.0);
  constellation += exp(-segmentDistance(uv, pointC, pointD) * 520.0);
  constellation += exp(-segmentDistance(uv, pointD, pointE) * 520.0);
  constellation += exp(-segmentDistance(uv, pointE, pointB) * 520.0);

  float bursts = starBurst(uv, pointA, 1.0, time) * 0.72;
  bursts += starBurst(uv, pointC, 0.82, time + 2.0) * 0.58;
  bursts += starBurst(uv, pointE, 0.68, time + 4.0) * 0.48;

  float shootingProgress = fract(time * 0.035 + 0.18);
  vec2 shootingHead = vec2(1.10 - shootingProgress * 0.88, 0.88 - shootingProgress * 0.47);
  vec2 shootingTail = shootingHead + vec2(0.16, 0.085);
  float shootingStar = exp(-segmentDistance(uv, shootingHead, shootingTail) * 310.0);
  shootingStar *= sin(shootingProgress * 3.1415926) * smoothstep(0.38, 0.72, uv.x);

  vec3 imageColor = imageBase;
  imageColor += warmGold * stars * fieldMask * 0.52;
  imageColor += paleLight * stars * fieldMask * 0.34;
  imageColor += warmGold * constellation * 0.13;
  imageColor += paleLight * bursts * 0.62;
  imageColor += paleLight * shootingStar * 0.42;

  vec3 base = mix(imageBase, videoBase, u_tone);
  vec3 color = mix(imageColor, videoColor, u_tone);
  float leftReadability = 1.0 - smoothstep(0.08, 0.58, uv.x);
  color = mix(color, base * 0.56, leftReadability * 0.76);
  float vignette = smoothstep(1.42, 0.20, length((uv - 0.5) * vec2(1.12, 0.82)));
  color *= 0.76 + vignette * 0.24;
  float grain = hash(floor(gl_FragCoord.xy) + floor(time * 3.0)) - 0.5;
  color += grain * 0.012;
  gl_FragColor = vec4(color, 1.0);
}
`

function createShader(type: number, source: string) {
  if (!gl) return null
  const shader = gl.createShader(type)
  if (!shader) return null
  gl.shaderSource(shader, source)
  gl.compileShader(shader)
  if (!gl.getShaderParameter(shader, gl.COMPILE_STATUS)) {
    console.error('Smoke shader compile failed:', gl.getShaderInfoLog(shader))
    gl.deleteShader(shader)
    return null
  }
  return shader
}

function setupWebGL() {
  const element = canvas.value
  if (!element) return false
  gl = element.getContext('webgl', { alpha: false, antialias: false, powerPreference: 'low-power' })
  if (!gl) return false
  const vertex = createShader(gl.VERTEX_SHADER, vertexSource)
  const fragment = createShader(gl.FRAGMENT_SHADER, fragmentSource)
  if (!vertex || !fragment) return false
  program = gl.createProgram()
  if (!program) return false
  gl.attachShader(program, vertex)
  gl.attachShader(program, fragment)
  gl.linkProgram(program)
  gl.deleteShader(vertex)
  gl.deleteShader(fragment)
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) {
    console.error('Smoke shader link failed:', gl.getProgramInfoLog(program))
    return false
  }
  gl.useProgram(program)
  const buffer = gl.createBuffer()
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 1, -1, -1, 1, -1, 1, 1, -1, 1, 1]), gl.STATIC_DRAW)
  const position = gl.getAttribLocation(program, 'a_position')
  gl.enableVertexAttribArray(position)
  gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0)
  resolutionUniform = gl.getUniformLocation(program, 'u_resolution')
  timeUniform = gl.getUniformLocation(program, 'u_time')
  toneUniform = gl.getUniformLocation(program, 'u_tone')
  gl.uniform1f(toneUniform, props.tone === 'video' ? 1 : 0)
  return true
}

function resize() {
  const element = canvas.value
  if (!element || !gl) return
  const rect = element.getBoundingClientRect()
  const ratio = Math.min(window.devicePixelRatio || 1, 1.5)
  element.width = Math.max(1, Math.round(rect.width * ratio))
  element.height = Math.max(1, Math.round(rect.height * ratio))
  gl.viewport(0, 0, element.width, element.height)
  render(performance.now(), true)
}

function render(now: number, force = false) {
  if (!gl || !program || !canvas.value) return
  if (!force && now - lastFrame < 32) {
    frame = window.requestAnimationFrame(render)
    return
  }
  lastFrame = now
  const elapsed = reducedMotion?.matches ? 11.5 : (now - startTime) / 1000
  gl.useProgram(program)
  gl.uniform2f(resolutionUniform, canvas.value.width, canvas.value.height)
  gl.uniform1f(timeUniform, elapsed)
  gl.drawArrays(gl.TRIANGLES, 0, 6)
  if (!force && !reducedMotion?.matches && visible && !document.hidden) frame = window.requestAnimationFrame(render)
}

function syncAnimation() {
  window.cancelAnimationFrame(frame)
  if (reducedMotion?.matches || !visible || document.hidden) {
    render(performance.now(), true)
    return
  }
  frame = window.requestAnimationFrame(render)
}

function handleContextLost(event: Event) {
  event.preventDefault()
  window.cancelAnimationFrame(frame)
}

function handleContextRestored() {
  setupWebGL()
  resize()
  syncAnimation()
}

onMounted(() => {
  if (!setupWebGL()) return
  startTime = performance.now() - (props.tone === 'video' ? 3700 : 0)
  reducedMotion = window.matchMedia('(prefers-reduced-motion: reduce)')
  reducedMotion.addEventListener('change', syncAnimation)
  resizeObserver = new ResizeObserver(resize)
  if (canvas.value) resizeObserver.observe(canvas.value)
  intersectionObserver = new IntersectionObserver(([entry]) => {
    visible = entry.isIntersecting
    syncAnimation()
  }, { threshold: .05 })
  if (canvas.value) {
    intersectionObserver.observe(canvas.value)
    canvas.value.addEventListener('webglcontextlost', handleContextLost)
    canvas.value.addEventListener('webglcontextrestored', handleContextRestored)
  }
  document.addEventListener('visibilitychange', syncAnimation)
  resize()
  syncAnimation()
})

onBeforeUnmount(() => {
  window.cancelAnimationFrame(frame)
  resizeObserver?.disconnect()
  intersectionObserver?.disconnect()
  reducedMotion?.removeEventListener('change', syncAnimation)
  document.removeEventListener('visibilitychange', syncAnimation)
  canvas.value?.removeEventListener('webglcontextlost', handleContextLost)
  canvas.value?.removeEventListener('webglcontextrestored', handleContextRestored)
  if (gl && program) gl.deleteProgram(program)
})
</script>

<template><canvas ref="canvas" class="model-atmosphere" aria-hidden="true" /></template>

<style scoped>
.model-atmosphere{position:absolute;inset:0;width:100%;height:100%;display:block;pointer-events:none}
</style>
