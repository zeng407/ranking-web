import { createApp } from 'vue'

import App from './App.vue'
import { initializeTheme } from './composables/useTheme'
import router from './router'
import './styles/main.css'

initializeTheme()
createApp(App).use(router).mount('#app')
