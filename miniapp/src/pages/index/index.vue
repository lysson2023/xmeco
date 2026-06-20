<template><view class="home">
  <view class="header"><text class="greet">XMECO</text><text class="sub">Energy Monitor</text></view>
  <view class="cards">
    <view class="card"><text class="val green">156</text><text class="lbl">Online</text></view>
    <view class="card"><text class="val yellow">3</text><text class="lbl">Alerts</text></view>
    <view class="card"><text class="val cyan">1245</text><text class="lbl">Saved(kWh)</text></view>
    <view class="card"><text class="val">28C</text><text class="lbl">Outdoor</text></view>
  </view>
  <view class="title">Quick Actions</view>
  <view class="actions">
    <view class="btn" @click="goDevices"><text>Devices</text></view>
    <view class="btn" @click="goAlarms"><text>Alarms</text></view>
    <view class="btn" @click="goMine"><text>Settings</text></view>
  </view>
</view></template>
<script setup lang="ts">
const goDevices = () => uni.switchTab({ url: '/pages/devices/devices' })
const goAlarms = () => uni.switchTab({ url: '/pages/alarms/alarms' })
const goMine = () => uni.switchTab({ url: '/pages/mine/mine' })
</script>
<style>
.home { padding: 20rpx; background: #f5f7fa; min-height: 100vh; }
.header { padding: 30rpx 0; text-align: center; }
.greet { font-size: 44rpx; font-weight: 700; color: #006875; display: block; }
.sub { font-size: 26rpx; color: #999; margin-top: 8rpx; }
.cards { display: flex; flex-wrap: wrap; gap: 16rpx; margin: 20rpx 0; }
.card { flex: 1; min-width: 40%; background: #fff; border-radius: 16rpx; padding: 30rpx; text-align: center; box-shadow: 0 2rpx 12rpx rgba(0,0,0,0.06); }
.val { font-size: 48rpx; font-weight: 700; display: block; }
.lbl { font-size: 24rpx; color: #999; margin-top: 8rpx; }
.green { color: #52c41a; } .yellow { color: #faad14; } .cyan { color: #006875; }
.title { font-size: 32rpx; font-weight: 600; margin: 30rpx 0 16rpx; }
.actions { display: flex; gap: 16rpx; }
.btn { flex: 1; background: #006875; color: #fff; text-align: center; padding: 24rpx; border-radius: 12rpx; font-size: 28rpx; }
</style>
