import { createApp } from 'vue'
import { createPinia } from 'pinia'
import App from './App.vue'
import router from './router'
import i18n, { initI18n } from './i18n'
import { useAppStore } from '@/stores/app'
import { safeLocalStorage } from '@/utils/browserStorage'
import './style.css'

function renderBootstrapError(title = '页面加载失败', detail = '请刷新页面重试，或清理浏览器缓存后再次访问。') {
  const appRoot = document.getElementById('app')
  if (!appRoot) {
    return
  }
  appRoot.innerHTML = `
    <div class="app-bootstrap-fallback">
      <div class="app-bootstrap-panel">
        <div class="app-bootstrap-kicker">ACCESS TERMINAL</div>
        <h1 class="app-bootstrap-title">${title}</h1>
        <p class="app-bootstrap-text">${detail}</p>
        <div class="app-bootstrap-bar"></div>
      </div>
    </div>
  `
}

function initThemeClass() {
  const savedTheme = safeLocalStorage.getItem('theme')
  const shouldUseDark = savedTheme === 'dark'
  document.documentElement.classList.toggle('dark', shouldUseDark)
}

async function bootstrap() {
  // Apply theme class globally before app mount to keep all routes consistent.
  initThemeClass()

  const app = createApp(App)
  const pinia = createPinia()
  app.use(pinia)

  // Initialize settings from injected config BEFORE mounting (prevents flash)
  // This must happen after pinia is installed but before router and i18n
  const appStore = useAppStore()
  appStore.initFromInjectedConfig()

  // Set document title immediately after config is loaded
  if (appStore.siteName && appStore.siteName !== 'Sub2API') {
    document.title = `${appStore.siteName} - AI API Gateway`
  }

  await initI18n()

  app.use(router)
  app.use(i18n)

  // 等待路由器完成初始导航后再挂载，避免竞态条件导致的空白渲染
  await router.isReady()
  app.mount('#app')
}

bootstrap().catch((error) => {
  console.error('Failed to bootstrap application:', error)
  renderBootstrapError()
})
