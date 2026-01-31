import { createRouter, createWebHashHistory } from "vue-router"
import MainPage from "./pages/MainPage.vue"
import SettingsPage from "./pages/SettingsPage.vue"

const router = createRouter({
  history: createWebHashHistory(),
  routes: [
    { path: "/", name: "Home", component: MainPage },
    { path: "/settings", name: "Settings", component: SettingsPage },
  ],
})

export default router
