<template>
<view class="login-page">
  <view class="bg-grid" />
  <view class="bg-glow" />
  <view class="login-card">
    <view class="brand">
      <text class="brand-title">XMECO</text>
      <text class="brand-sub">Multi-Intelligence Energy Efficiency System</text>
    </view>
    <view class="form">
      <view class="field">
        <text class="label">Username</text>
        <input class="input" v-model="username" placeholder="Enter username" placeholder-style="color:rgba(255,255,255,0.3)" />
      </view>
      <view class="field">
        <text class="label">Password</text>
        <input class="input" v-model="password" placeholder="Enter password" password placeholder-style="color:rgba(255,255,255,0.3)" />
      </view>
      <button class="login-btn" :loading="loading" @click="doLogin">{{ loading ? 'Logging in...' : 'Login' }}</button>
    </view>
    <text class="footer-text">Shenzhen High Altitude Technology Co., Ltd.</text>
  </view>
</view>
</template>
<script setup lang="ts">
import { ref } from 'vue';
import { api } from '../../api/client';
const username = ref('admin'), password = ref('admin123'), loading = ref(false);
const doLogin = async () => {
  loading.value = true;
  try {
    const d = await api.login(username.value, password.value) as any;
    uni.setStorageSync('token', d.token); uni.setStorageSync('user', JSON.stringify(d.user));
    uni.switchTab({ url: '/pages/index/index' });
  } catch(e) { uni.showToast({ title: 'Login failed', icon: 'none' }); }
  finally { loading.value = false; }
};
</script>
<style>
page { background: linear-gradient(160deg, #001a1f, #003740, #00252e); min-height: 100vh; }
.login-page { display: flex; align-items: center; justify-content: center; min-height: 100vh; position: relative; overflow: hidden; padding: 40rpx; }
.bg-grid {
  position: absolute; inset: 0; opacity: 0.08;
  background-image: linear-gradient(rgba(255,255,255,0.3) 1px, transparent 1px), linear-gradient(90deg, rgba(255,255,255,0.3) 1px, transparent 1px);
  background-size: 40rpx 40rpx;
}
.bg-glow {
  position: absolute; width: 600rpx; height: 600rpx; border-radius: 50%;
  background: radial-gradient(circle, rgba(0,200,220,0.15), transparent);
  top: 50%; left: 50%; transform: translate(-50%, -50%);
}
.login-card {
  position: relative; z-index: 1; width: 100%; max-width: 600rpx;
  background: rgba(255,255,255,0.06); backdrop-filter: blur(30rpx);
  border-radius: 40rpx; padding: 60rpx 50rpx;
  box-shadow: 0 20rpx 60rpx rgba(0,0,0,0.3), 0 0 80rpx rgba(0,200,220,0.1);
}
.brand { text-align: center; margin-bottom: 60rpx; }
.brand-title { font-size: 48rpx; font-weight: 700; color: #67e8f9; display: block; letter-spacing: 4rpx; }
.brand-sub { font-size: 22rpx; color: rgba(103,232,249,0.5); margin-top: 12rpx; display: block; letter-spacing: 2rpx; }
.field { margin-bottom: 32rpx; }
.label { font-size: 22rpx; color: rgba(103,232,249,0.5); margin-bottom: 12rpx; display: block; letter-spacing: 2rpx; }
.input {
  width: 100%; background: rgba(255,255,255,0.06); border-radius: 16rpx; padding: 24rpx 28rpx;
  font-size: 30rpx; color: #fff; border: 1rpx solid rgba(103,232,249,0.15);
}
.login-btn {
  width: 100%; background: linear-gradient(135deg, #0891b2, #06b6d4, #22d3ee); border-radius: 20rpx;
  color: #001a1f; font-size: 32rpx; font-weight: 700; padding: 28rpx; margin-top: 20rpx; border: none;
  box-shadow: 0 8rpx 30rpx rgba(6,182,212,0.3);
}
.login-btn::after { border: none; }
.footer-text { text-align: center; font-size: 20rpx; color: rgba(103,232,249,0.25); margin-top: 40rpx; letter-spacing: 2rpx; }
</style>