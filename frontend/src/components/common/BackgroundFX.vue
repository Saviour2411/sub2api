<template>
  <div
    :class="['background-fx', `background-fx-${variant}`, { 'pointer-events-none': true }]"
    aria-hidden="true"
  >
    <!-- Canvas particle field (only when motion allowed) -->
    <canvas v-if="canAnimate" ref="canvasEl" class="background-fx-canvas" />

    <!-- Static gradient overlay (always shown for depth) -->
    <div class="bg-veil"></div>

    <!-- Diagonal energy beams -->
    <div v-if="canAnimate" class="bg-beam bg-beam-1"></div>
    <div v-if="canAnimate" class="bg-beam bg-beam-2"></div>

    <!-- Slow rotating HUD reticle (decoration) -->
    <div v-if="variant === 'home' || variant === 'auth'" class="bg-reticle mecha-target-spin">
      <div class="bg-reticle-inner mecha-target-spin-rev"></div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref, watch } from 'vue'

type Variant = 'app' | 'home' | 'auth'

interface Props {
  variant?: Variant
  /** Multiplier on particle count (default 1) */
  density?: number
}

const props = withDefaults(defineProps<Props>(), {
  variant: 'app',
  density: 1
})

const canvasEl = ref<HTMLCanvasElement | null>(null)
let ctx: CanvasRenderingContext2D | null = null
let rafId = 0
let particles: Particle[] = []
let dpr = 1
let width = 0
let height = 0
let lastTs = 0
let isHidden = false
const motionPref = ref<boolean>(false)
const isMobile = ref<boolean>(false)

interface Particle {
  x: number
  y: number
  vx: number
  vy: number
  r: number
  baseAlpha: number
  hue: number
  twinkleOffset: number
}

const canAnimate = computed(() => !motionPref.value)

function getBaseCount(): number {
  const map: Record<Variant, number> = { app: 38, home: 70, auth: 64 }
  return map[props.variant] ?? 40
}

function getColors(): string[] {
  // Hue-style colors, mapped to bluish + warm orange
  if (props.variant === 'home' || props.variant === 'auth') {
    return ['203, 86%, 64%', '193, 100%, 78%', '24, 92%, 62%']
  }
  return ['203, 86%, 60%', '193, 100%, 76%']
}

function configureCanvas() {
  const canvas = canvasEl.value
  if (!canvas) return
  dpr = Math.min(window.devicePixelRatio || 1, 2)
  width = canvas.clientWidth
  height = canvas.clientHeight
  canvas.width = Math.max(1, Math.floor(width * dpr))
  canvas.height = Math.max(1, Math.floor(height * dpr))
  ctx = canvas.getContext('2d')
  if (ctx) {
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0)
  }
}

function buildParticles() {
  const count = Math.max(8, Math.floor(getBaseCount() * (props.density ?? 1) * (isMobile.value ? 0.5 : 1)))
  particles = []
  for (let i = 0; i < count; i++) {
    particles.push(spawnParticle(i))
  }
}

function spawnParticle(seed = 0): Particle {
  const colors = getColors()
  return {
    x: Math.random() * width,
    y: Math.random() * height,
    vx: (Math.random() - 0.5) * 0.16,
    vy: -0.06 - Math.random() * 0.18,
    r: 0.6 + Math.random() * 1.6,
    baseAlpha: 0.18 + Math.random() * 0.42,
    hue: parseFloat(String((seed % colors.length))),
    twinkleOffset: Math.random() * Math.PI * 2
  }
}

function step(ts: number) {
  if (!ctx || !canAnimate.value) {
    rafId = 0
    return
  }
  const dt = Math.min(48, ts - (lastTs || ts))
  lastTs = ts

  ctx.clearRect(0, 0, width, height)

  // Faint connecting grid drift (very subtle): radial gradient hotspot
  const colors = getColors()

  for (let i = 0; i < particles.length; i++) {
    const p = particles[i]
    p.x += p.vx * (dt * 0.06)
    p.y += p.vy * (dt * 0.06)
    if (p.y < -8) {
      p.y = height + 8
      p.x = Math.random() * width
    }
    if (p.x < -8) p.x = width + 8
    if (p.x > width + 8) p.x = -8

    const twinkle = (Math.sin(ts * 0.0015 + p.twinkleOffset) + 1) / 2
    const alpha = p.baseAlpha * (0.55 + twinkle * 0.45)
    const colorIdx = Math.floor(p.hue) % colors.length
    const color = colors[colorIdx]

    ctx.beginPath()
    ctx.arc(p.x, p.y, p.r, 0, Math.PI * 2)
    ctx.fillStyle = `hsla(${color}, ${alpha.toFixed(3)})`
    ctx.shadowColor = `hsla(${color}, ${(alpha * 0.7).toFixed(3)})`
    ctx.shadowBlur = 8
    ctx.fill()
  }

  // Connect close pairs with thin lines (mesh feel)
  ctx.shadowBlur = 0
  ctx.lineWidth = 0.5
  for (let i = 0; i < particles.length; i++) {
    for (let j = i + 1; j < particles.length; j++) {
      const a = particles[i]
      const b = particles[j]
      const dx = a.x - b.x
      const dy = a.y - b.y
      const dist2 = dx * dx + dy * dy
      if (dist2 < 11000) {
        const alpha = (1 - dist2 / 11000) * 0.18
        ctx.strokeStyle = `hsla(${getColors()[0]}, ${alpha.toFixed(3)})`
        ctx.beginPath()
        ctx.moveTo(a.x, a.y)
        ctx.lineTo(b.x, b.y)
        ctx.stroke()
      }
    }
  }

  rafId = requestAnimationFrame(step)
}

function onResize() {
  configureCanvas()
  buildParticles()
}

function onVisibility() {
  isHidden = document.hidden
  if (isHidden) {
    if (rafId) cancelAnimationFrame(rafId)
    rafId = 0
  } else if (canAnimate.value && !rafId) {
    lastTs = 0
    rafId = requestAnimationFrame(step)
  }
}

function detectPreferences() {
  motionPref.value = window.matchMedia?.('(prefers-reduced-motion: reduce)').matches ?? false
  isMobile.value = window.matchMedia?.('(max-width: 768px)').matches ?? false
}

let mediaList: MediaQueryList | null = null
let widthList: MediaQueryList | null = null
function onMediaChange() {
  const wasAnimating = canAnimate.value
  detectPreferences()
  if (canAnimate.value && !wasAnimating) {
    // Re-init
    requestAnimationFrame(() => {
      configureCanvas()
      buildParticles()
      if (!rafId) {
        lastTs = 0
        rafId = requestAnimationFrame(step)
      }
    })
  } else if (!canAnimate.value && rafId) {
    cancelAnimationFrame(rafId)
    rafId = 0
  }
}

onMounted(() => {
  detectPreferences()
  if (canAnimate.value) {
    // Wait a frame so canvas has measured layout
    requestAnimationFrame(() => {
      configureCanvas()
      buildParticles()
      lastTs = 0
      rafId = requestAnimationFrame(step)
    })
  }
  window.addEventListener('resize', onResize, { passive: true })
  document.addEventListener('visibilitychange', onVisibility)
  if (window.matchMedia) {
    mediaList = window.matchMedia('(prefers-reduced-motion: reduce)')
    widthList = window.matchMedia('(max-width: 768px)')
    mediaList.addEventListener?.('change', onMediaChange)
    widthList.addEventListener?.('change', onMediaChange)
  }
})

watch(() => props.density, () => {
  if (canAnimate.value) buildParticles()
})

watch(() => props.variant, () => {
  if (canAnimate.value) buildParticles()
})

onBeforeUnmount(() => {
  if (rafId) cancelAnimationFrame(rafId)
  rafId = 0
  window.removeEventListener('resize', onResize)
  document.removeEventListener('visibilitychange', onVisibility)
  mediaList?.removeEventListener?.('change', onMediaChange)
  widthList?.removeEventListener?.('change', onMediaChange)
})
</script>

<style scoped>
.background-fx {
  position: fixed;
  inset: 0;
  z-index: 0;
  pointer-events: none;
  overflow: hidden;
}

.bg-veil {
  position: absolute;
  inset: 0;
  pointer-events: none;
  background:
    radial-gradient(ellipse 80% 60% at 80% 0%, rgba(75, 181, 255, 0.18), transparent 60%),
    radial-gradient(ellipse 60% 50% at 0% 100%, rgba(255, 111, 56, 0.1), transparent 60%);
}

.background-fx-app .bg-veil {
  background:
    radial-gradient(ellipse 80% 60% at 100% 0%, rgba(75, 181, 255, 0.14), transparent 56%),
    radial-gradient(ellipse 50% 40% at 0% 110%, rgba(255, 111, 56, 0.08), transparent 60%);
}

.background-fx-auth .bg-veil {
  background:
    radial-gradient(ellipse 60% 60% at 30% 30%, rgba(75, 181, 255, 0.22), transparent 56%),
    radial-gradient(ellipse 50% 60% at 80% 70%, rgba(255, 111, 56, 0.16), transparent 60%);
}

.bg-beam {
  position: absolute;
  width: 200%;
  height: 110px;
  left: -50%;
  pointer-events: none;
  background: linear-gradient(
    90deg,
    transparent 0%,
    rgba(75, 181, 255, 0.0) 30%,
    rgba(75, 181, 255, 0.16) 48%,
    rgba(255, 255, 255, 0.32) 50%,
    rgba(255, 111, 56, 0.16) 52%,
    rgba(255, 111, 56, 0) 70%,
    transparent 100%
  );
  filter: blur(2px);
  mix-blend-mode: screen;
  opacity: 0.55;
}

.bg-beam-1 {
  top: 22%;
  transform: rotate(-12deg);
  animation: bg-beam-drift-1 14s ease-in-out infinite;
}

.bg-beam-2 {
  top: 64%;
  transform: rotate(8deg);
  animation: bg-beam-drift-2 22s ease-in-out infinite;
  opacity: 0.4;
}

@keyframes bg-beam-drift-1 {
  0%, 100% { transform: translateY(-30px) rotate(-12deg); opacity: 0.4; }
  50% { transform: translateY(40px) rotate(-9deg); opacity: 0.7; }
}

@keyframes bg-beam-drift-2 {
  0%, 100% { transform: translateY(20px) rotate(8deg); opacity: 0.3; }
  50% { transform: translateY(-30px) rotate(11deg); opacity: 0.55; }
}

.bg-reticle {
  position: absolute;
  width: 360px;
  height: 360px;
  right: -90px;
  top: 12vh;
  border: 1px solid rgba(75, 181, 255, 0.2);
  border-radius: 50%;
  pointer-events: none;
}

.bg-reticle::before,
.bg-reticle::after {
  content: '';
  position: absolute;
  inset: 22px;
  border: 1px dashed rgba(75, 181, 255, 0.16);
  border-radius: 50%;
}

.bg-reticle::after {
  inset: 64px;
  border-style: solid;
  border-color: rgba(255, 111, 56, 0.18);
}

.bg-reticle-inner {
  position: absolute;
  inset: 100px;
  border: 1px solid rgba(75, 181, 255, 0.3);
  border-radius: 50%;
}

.bg-reticle-inner::before {
  content: '';
  position: absolute;
  left: 50%;
  top: 0;
  bottom: 0;
  width: 1px;
  background: linear-gradient(180deg, transparent, rgba(75, 181, 255, 0.5), transparent);
}

.background-fx-app .bg-reticle {
  display: none;
}

.background-fx-home .bg-reticle {
  right: -120px;
  top: 6vh;
  width: 480px;
  height: 480px;
  opacity: 0.7;
}

.background-fx-auth .bg-reticle {
  right: 4vw;
  top: 12vh;
  width: 420px;
  height: 420px;
  opacity: 0.55;
}

@media (max-width: 768px) {
  .bg-reticle {
    display: none;
  }
  .bg-beam {
    height: 80px;
    opacity: 0.35;
  }
}

@media (prefers-reduced-motion: reduce) {
  .bg-beam,
  .bg-reticle,
  .bg-reticle-inner {
    animation: none !important;
  }
}
</style>
