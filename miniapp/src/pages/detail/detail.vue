<template><view class="detail">
  <view class="head"><text class="name">{{dev.name}}</text><text class="type">{{dev.device_type}}</text></view>
  <view class="props" v-for="p in props" :key="p.id">
    <text class="pn">{{p.prop_name}}</text><text class="pv">{{p.prop_value}} {{p.unit}}</text>
  </view>
  <view class="controls">
    <button class="ctrl on" @click="control('start')">Start</button>
    <button class="ctrl off" @click="control('stop')">Stop</button>
  </view>
</view></template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'; import { api } from '../../api/client';
const dev = ref({} as any); const props = ref([] as any[])
onMounted(async () => {
  const pages = getCurrentPages(); const id = (pages[pages.length-1] as any).options.id
  const [d, p] = await Promise.all([api.get('/devices/'+id), api.get('/properties?device_id='+id)])
  dev.value = d.data as any; props.value = p.data as any
})
const control = async (a: string) => { await api.post('/devices/'+dev.value.id+'/control', { action: a }); uni.showToast({ title: 'OK' }) }
</script>
<style>
.detail { padding: 20rpx; }
.head { padding: 30rpx; background: #006875; color: #fff; border-radius: 16rpx; margin-bottom: 20rpx; }
.name { font-size: 36rpx; font-weight: 700; display: block; }
.type { font-size: 24rpx; opacity: 0.8; }
.props { display: flex; justify-content: space-between; background: #fff; padding: 24rpx; margin-bottom: 8rpx; border-radius: 8rpx; }
.pn { font-size: 28rpx; color: #333; } .pv { font-size: 28rpx; font-weight: 600; color: #006875; }
.controls { display: flex; gap: 20rpx; margin-top: 30rpx; }
.ctrl { flex: 1; border-radius: 12rpx; font-size: 30rpx; }
.on { background: #52c41a; color: #fff; } .off { background: #ff4d4f; color: #fff; }
</style>
