import { createRouter, createWebHistory, type RouteRecordRaw } from 'vue-router'

/**
 * The back office's routes.
 *
 * History base is `/admin/`, which is both where Go mounts the bundle and the Path of the
 * pass cookie. A route added outside that prefix would navigate away from the gate and
 * land on the public SPA.
 *
 * These routes are NEVER added to the public router: a link there would put the admin
 * chunks in the public build, which is the one thing this split exists to prevent.
 */
const routes: RouteRecordRaw[] = [
  { path: '/', redirect: '/posts' },
  { path: '/posts', name: 'posts', component: () => import('./views/PostsView.vue') },
  {
    path: '/posts/:serial/elements',
    name: 'post-elements',
    component: () => import('./views/PostElementsView.vue'),
  },
  { path: '/users', name: 'users', component: () => import('./views/UsersView.vue') },
  { path: '/carousel', name: 'carousel', component: () => import('./views/CarouselView.vue') },
  {
    path: '/announcement',
    name: 'announcement',
    component: () => import('./views/AnnouncementView.vue'),
  },
  { path: '/:pathMatch(.*)*', redirect: '/posts' },
]

export default createRouter({
  history: createWebHistory('/admin/'),
  routes,
})
