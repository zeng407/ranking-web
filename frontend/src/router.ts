import {
  createRouter,
  createWebHistory,
  type RouteLocationGeneric,
  type RouteLocationRaw,
  type RouteRecordRaw,
} from 'vue-router'

import { localeCodePattern, localePrefixPattern, localizedPath, normalizeLocale } from './i18n'

const HomeView = () => import('./views/HomeView.vue')
const DonateView = () => import('./views/DonateView.vue')
const LegalView = () => import('./views/LegalView.vue')
const GameView = () => import('./views/GameView.vue')
const LoginView = () => import('./views/LoginView.vue')
const GameRoomView = () => import('./views/GameRoomView.vue')
const AccountView = () => import('./views/AccountView.vue')
const MyPostsView = () => import('./views/MyPostsView.vue')
const PostEditorView = () => import('./views/PostEditorView.vue')

function localizedPublicRoutes(): RouteRecordRaw[] {
  // Built from the locale registry in i18n.ts; adding a language needs no
  // change here.
  const prefix = `/:locale(${localePrefixPattern})`

  return [
    { path: `${prefix}/login`, name: 'login-localized', component: LoginView },
    { path: `${prefix}/`, name: 'home-localized', component: HomeView, meta: { viewKey: 'home', keepAlive: true } },
    { path: `${prefix}/hot`, name: 'hot-localized', component: HomeView, meta: { viewKey: 'home', keepAlive: true } },
    { path: `${prefix}/new`, name: 'new-localized', component: HomeView, meta: { viewKey: 'home', keepAlive: true } },
    { path: `${prefix}/g/:serial`, name: 'game-localized', component: GameView },
    // The room is its own route rather than a mode of the game view: a participant is
    // watching somebody else's game and never runs the local game state machine.
    { path: `${prefix}/room/:serial`, name: 'room-localized', component: GameRoomView },
    { path: `${prefix}/r/:serial`, name: 'rank-localized', component: GameView },
    { path: `${prefix}/donate`, name: 'donate-localized', component: DonateView },
    // Localized only: a settings page is read, so it belongs on a URL that says which
    // language it is in, the same as every other page a signed-in user lands on.
    { path: `${prefix}/account`, name: 'account-localized', component: AccountView },
    { path: `${prefix}/account/posts`, name: 'my-posts-localized', component: MyPostsView },
    {
      path: `${prefix}/account/posts/:serial`,
      name: 'post-editor-localized',
      component: PostEditorView,
    },
    {
      path: `${prefix}/tos`,
      name: 'tos-localized',
      component: LegalView,
      props: { document: 'tos' },
    },
    {
      path: `${prefix}/privacy`,
      name: 'privacy-localized',
      component: LegalView,
      props: { document: 'privacy' },
    },
  ]
}

function redirectToPath(to: RouteLocationGeneric, path: string): RouteLocationRaw {
  return { path, query: to.query, hash: to.hash }
}

function legacyLocaleRedirect(to: RouteLocationGeneric): RouteLocationRaw {
  const locale = normalizeLocale(to.params.legacyLocale)
  const legacyPath = to.params.pathMatch
  const path = Array.isArray(legacyPath) ? legacyPath.join('/') : String(legacyPath || '')
  return redirectToPath(to, localizedPath(path ? `/${path}` : '/', locale))
}

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'home', component: HomeView, meta: { viewKey: 'home', keepAlive: true } },
    { path: '/hot', name: 'hot', component: HomeView, meta: { viewKey: 'home', keepAlive: true } },
    { path: '/new', name: 'new', component: HomeView, meta: { viewKey: 'home', keepAlive: true } },
    { path: '/g/:serial', name: 'game', component: GameView },
    { path: '/room/:serial', name: 'room', component: GameRoomView },
    { path: '/r/:serial', name: 'rank', component: GameView },
    { path: '/login', name: 'login', component: LoginView },
    { path: '/donate', redirect: (to) => redirectToPath(to, '/zh-tw/donate') },
    { path: '/tos', redirect: (to) => redirectToPath(to, '/zh-tw/tos') },
    { path: '/privacy', redirect: (to) => redirectToPath(to, '/zh-tw/privacy') },
    ...localizedPublicRoutes(),
    {
      path: `/lang/:legacyLocale(${localeCodePattern})`,
      redirect: legacyLocaleRedirect,
    },
    {
      path: `/lang/:legacyLocale(${localeCodePattern})/:pathMatch(.*)*`,
      redirect: legacyLocaleRedirect,
    },
    {
      path: '/migration',
      name: 'migration',
      component: () => import('./views/MigrationView.vue'),
    },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior: (to, from, savedPosition) => {
    if (savedPosition) return savedPosition
    // Searching and selecting a tag only changes `?k=`. Keep the list in view
    // instead of jumping back to the carousel at the top of the home page.
    if (to.path === from.path || (to.meta.viewKey && to.meta.viewKey === from.meta.viewKey)) return false
    return { top: 0 }
  },
})

export default router
