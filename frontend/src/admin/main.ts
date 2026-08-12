import { createApp } from 'vue'

import AdminApp from './AdminApp.vue'
import router from './router'
import { initializeTheme } from '../composables/useTheme'
import './styles.css'

initializeTheme()
createApp(AdminApp).use(router).mount('#admin')
