<template><view class="list">
  <view class="filters" v-if="deviceId">
    <text class="filter-tag">设备ID: {{deviceId}}</text>
  </view>
  <view class="item" v-for="a in alarms" :key="a.id">
    <view class="info"><text class="msg">{{a.message}}</text><text class="ts">{{a.created_at?.slice(0,19)}}</text></view>
    <text class="level" :class="a.level">{{a.level}}</text>
  </view>
</view></template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'; import { api } from '../../api/client';
import { AuthError } from '../../api/client';
const alarms = ref([] as any[])
const deviceId = ref('')
onMounted(async () => {
  try {
    const pages = getCurrentPages();
    const opts = (pages[pages.length-1] as any).options || {};
    deviceId.value = opts.device_id || '';
    let path = '/alarm-logs';
    if (deviceId.value) path += '?device_id=' + encodeURIComponent(deviceId.value);
    const r = await api.get(path);
    alarms.value = (r.data as any) || []
  } catch (e) { if (!(e instanceof AuthError)) uni.showToast({ title: '告警加载失败', icon: 'none' }) }
})
</script>
<style>
.list { padding: 20rpx; }
.filters { margin-bottom: 12rpx; }
.filter-tag { font-size: 22rpx; color: #006875; background: #e6f7f7; padding: 6rpx 16rpx; border-radius: 16rpx; }
.item { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 24rpx; margin-bottom: 12rpx; border-radius: 12rpx; }
.msg { font-size: 28rpx; display: block; } .ts { font-size: 22rpx; color: #999; }
.level { font-size: 22rpx; padding: 4rpx 16rpx; border-radius: 8rpx; }
.warning { background: #fff7e6; color: #faad14; } .critical { background: #fff1f0; color: #ff4d4f; }
</style>
