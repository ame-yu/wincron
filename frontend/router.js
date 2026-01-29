import { createRouter, createWebHashHistory } from "vue-router"
import MainPage from "./pages/MainPage.vue"
import SettingsPage from "./pages/SettingsPage.vue"

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: "/", name: "main", component: MainPage },
    { path: "/settings", name: "settings", component: SettingsPage },
  ],
})

export default router
