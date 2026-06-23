<template><view class="mine">
  <view class="head"><text class="username">{{userName}}</text><text class="role">{{roleName}}</text></view>
  <view class="menu">
    <view class="item" @click="logout"><text>退出登录</text></view>
  </view>
</view></template>
<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { api } from '../../api/client';
const userName = ref(''), roleName = ref('');
onMounted(async () => {
  try {
    const r = await api.get('/auth/me');
    const d = r.data as any;
    userName.value = d.username || '未登录';
    roleName.value = d.role || '';
  } catch {}
});
const logout = () => { uni.removeStorageSync('token'); uni.reLaunch({ url: '/pages/login/login' }) }
</script>
<style>
.mine { padding: 20rpx; }
.head { padding: 40rpx; background: #006875; color: #fff; border-radius: 16rpx; text-align: center; margin-bottom: 20rpx; }
.username { font-size: 36rpx; font-weight: 700; display: block; }
.role { font-size: 24rpx; opacity: 0.8; }
.menu { background: #fff; border-radius: 12rpx; }
.item { padding: 30rpx; text-align: center; border-bottom: 1rpx solid #f0f0f0; font-size: 30rpx; }
</style>
