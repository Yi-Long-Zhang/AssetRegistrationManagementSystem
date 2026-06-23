import { ref } from 'vue'
import { defineStore } from 'pinia'

export const useAppStore = defineStore('app', () => {
  const sidebarCollapsed = ref(false)
  const pageTitle = ref('资产管理系统')
  const globalLoading = ref(false)

  function toggleSidebar() {
    sidebarCollapsed.value = !sidebarCollapsed.value
  }

  function setPageTitle(title) {
    pageTitle.value = title || '资产管理系统'
    document.title = `${pageTitle.value} - 资产管理系统`
  }

  return { sidebarCollapsed, pageTitle, globalLoading, toggleSidebar, setPageTitle }
})
