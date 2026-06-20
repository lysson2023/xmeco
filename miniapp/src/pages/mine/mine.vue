<template><view class="mine">
  <view class="head"><text class="username">Admin</text><text class="role">Super Admin</text></view>
  <view class="menu">
    <view class="item" @click="logout"><text>Logout</text></view>
  </view>
</view></template>
<script setup lang="ts">
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
