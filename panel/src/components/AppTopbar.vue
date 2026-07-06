<script setup lang="ts">
import { NAvatar, NButton, NDropdown } from 'naive-ui'

type DropdownOption = {
  label: string
  key: string
  disabled?: boolean
}

defineProps<{
  siteName: string
  siteDescription: string
  siteAvatar: string
  siteInitial: string
  userAvatar: string
  isLoggedIn: boolean
  showAccountMenu: boolean
  currentUser: string
  dropdownOptions: DropdownOption[]
}>()

const emit = defineEmits<{
  (event: 'open-assets'): void
  (event: 'login'): void
  (event: 'user-action', key: string): void
}>()
</script>

<template>
  <header class="topbar">
    <div class="brand">
      <div class="logo" :class="{ 'has-image': siteAvatar }" aria-hidden="true">
        <img v-if="siteAvatar" :src="siteAvatar" :alt="siteName" />
        <span v-else>{{ siteInitial }}</span>
      </div>
      <div class="brand-title">
        <strong>{{ siteName }}</strong>
        <span v-if="siteDescription">{{ siteDescription }}</span>
      </div>
    </div>

    <div class="actions" aria-label="账户操作">
      <button v-if="isLoggedIn" class="avatar-btn funds-btn" type="button" aria-label="查看资产统计" @click="emit('open-assets')">
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M6.2 7.4c0-1.55 2.55-2.8 5.8-2.8s5.8 1.25 5.8 2.8-2.55 2.8-5.8 2.8-5.8-1.25-5.8-2.8Z" />
          <path d="M6.2 7.4v5.1c0 1.55 2.55 2.8 5.8 2.8s5.8-1.25 5.8-2.8V7.4" />
          <path d="M6.2 12.5v3.6c0 1.55 2.55 2.8 5.8 2.8s5.8-1.25 5.8-2.8v-3.6" />
          <path d="M9 14.5l2-2 1.9 1.5 2.8-3.2" />
        </svg>
      </button>

      <n-dropdown
        v-if="showAccountMenu && isLoggedIn"
        :options="dropdownOptions"
        placement="bottom-end"
        @select="(key: string | number) => emit('user-action', String(key))"
      >
        <button class="avatar-btn" type="button" aria-label="打开用户菜单">
          <n-avatar v-if="userAvatar" round size="medium" :src="userAvatar" :alt="currentUser" />
          <n-avatar v-else round size="medium">{{ currentUser.slice(0, 1).toUpperCase() }}</n-avatar>
        </button>
      </n-dropdown>
      <button v-else-if="showAccountMenu" class="avatar-btn" type="button" aria-label="登录" @click="emit('login')">
        <n-avatar round size="medium">A</n-avatar>
      </button>
      <n-button v-if="showAccountMenu && !isLoggedIn" type="primary" @click="emit('login')">登录</n-button>
    </div>
  </header>
</template>
