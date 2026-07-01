<template><view class="mine">
  <view class="loading" v-if="loading">加载中...</view>
  <template v-else>
    <view class="head"><text class="username">{{userName}}</text><text class="role">{{roleName}}</text></view>
    <view class="menu">
      <view class="item" @click="logout"><text>退出登录</text></view>
    </view>
  </template>
</view></template>
<script setup lang="ts">
import { ref, onShow } from 'vue';
import { api, clearToken } from '../../api/client';
import { AuthError } from '../../api/client';
const userName = ref(''), roleName = ref(''), loading = ref(true);

const fetchUserInfo = async () => {
  loading.value = true;
  try {
    const r = await api.get('/auth/me');
    const d = r.data as any;
    userName.value = d.username || '未登录';
    roleName.value = d.role_name || d.role || '';
  } catch (e) {
    if (!(e instanceof AuthError)) {
      uni.showToast({ title: '用户信息加载失败', icon: 'none' });
    }
  } finally { loading.value = false }
};

onShow(() => { fetchUserInfo(); });

const confirmLogout = () => {
  uni.showModal({
    title: '退出登录',
    content: '确定要退出登录吗？',
    success: (res) => { if (res.confirm) { clearToken(); uni.reLaunch({ url: '/pages/login/login' }) } }
  });
};
// Keep original logout reference for template compatibility
const logout = confirmLogout;
</script>
<style>
.mine { padding: 20rpx; }
.loading { text-align: center; padding: 80rpx; color: #999; font-size: 28rpx; }
.head { padding: 40rpx; background: #006875; color: #fff; border-radius: 16rpx; text-align: center; margin-bottom: 20rpx; }
.username { font-size: 36rpx; font-weight: 700; display: block; }
.role { font-size: 24rpx; opacity: 0.8; }
.menu { background: #fff; border-radius: 12rpx; }
.item { padding: 30rpx; text-align: center; border-bottom: 1rpx solid #f0f0f0; font-size: 30rpx; }
</style>
