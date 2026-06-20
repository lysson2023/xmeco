<template><view class="list">
  <view class="item" v-for="a in alarms" :key="a.ID">
    <view class="info"><text class="msg">{{a.Msg}}</text><text class="ts">{{a.Ts?.slice(0,19)}}</text></view>
    <text class="level" :class="a.Level">{{a.Level}}</text>
  </view>
</view></template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'; import { api } from '../../api/client';
const alarms = ref([] as any[])
onMounted(async () => { try { const r = await api.get('/alarm-logs'); alarms.value = (r.data as any) } catch {} })
</script>
<style>
.list { padding: 20rpx; }
.item { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 24rpx; margin-bottom: 12rpx; border-radius: 12rpx; }
.msg { font-size: 28rpx; display: block; } .ts { font-size: 22rpx; color: #999; }
.level { font-size: 22rpx; padding: 4rpx 16rpx; border-radius: 8rpx; }
.warning { background: #fff7e6; color: #faad14; } .critical { background: #fff1f0; color: #ff4d4f; }
</style>
