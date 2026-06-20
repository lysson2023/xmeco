<template><view class="login">
  <text class="title">XMECO</text>
  <input class="inp" v-model="username" placeholder="Username" />
  <input class="inp" v-model="password" placeholder="Password" password />
  <button class="btn" @click="doLogin">Login</button>
</view></template>
<script setup lang="ts">
import { ref } from 'vue'; import { api } from '../../api/client';
const username = ref(''); const password = ref('')
const doLogin = async () => {
  try { await api.login(username.value, password.value); uni.switchTab({ url: '/pages/index/index' }) }
  catch { uni.showToast({ title: 'Login failed', icon: 'none' }) }
}
</script>
<style>
.login { padding: 80rpx 40rpx; }
.title { font-size: 48rpx; font-weight: 700; color: #006875; text-align: center; display: block; margin-bottom: 60rpx; }
.inp { border: 1rpx solid #ddd; border-radius: 12rpx; padding: 24rpx; margin-bottom: 20rpx; font-size: 30rpx; }
.btn { background: #006875; color: #fff; border-radius: 12rpx; font-size: 32rpx; margin-top: 40rpx; }
</style>
