<template><view class="list">
  <view class="item" v-for="d in devices" :key="d.id" @click="goDetail(d)">
    <view class="info"><text class="name">{{d.name}}</text><text class="type">{{d.device_type}}</text></view>
    <text class="status" :class="d.online_status==='online'?'on':'off'">{{d.online_status||'offline'}}</text>
  </view>
</view></template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'; import { api } from '../../api/client';
const devices = ref([] as any[])
onMounted(async () => { try { const r = await api.get('/devices'); devices.value = (r.data as any) } catch {} })
const goDetail = (d: any) => uni.navigateTo({ url: '/pages/detail/detail?id='+d.id })
</script>
<style>
.list { padding: 20rpx; }
.item { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 24rpx; margin-bottom: 12rpx; border-radius: 12rpx; }
.name { font-size: 30rpx; font-weight: 600; display: block; }
.type { font-size: 24rpx; color: #999; }
.status { font-size: 24rpx; } .on { color: #52c41a; } .off { color: #999; }
</style>
