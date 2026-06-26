<template><view class="list">
  <view class="loading" v-if="loading">加载中...</view>
  <template v-else>
    <view class="filter-bar">
      <picker mode="selector" :range="buildingNames" @change="onBldChange">
        <view class="picker">{{selBuildingName||'全部楼宇'}}</view>
      </picker>
      <text class="count" v-if="devices.length">{{devices.length}}台</text>
    </view>
    <view class="empty" v-if="!devices.length">暂无设备</view>
    <view class="item" v-for="d in devices" :key="d.id">
      <view class="info" @click="goDetail(d)"><text class="name">{{d.name}}</text><text class="type">{{d.device_type}}</text></view>
      <text class="history-link" @click="goHistory(d)">历史</text>
      <text class="status" :class="d.online_status==='在线'?'on':'off'">{{d.online_status||'离线'}}</text>
    </view>
  </template>
</view></template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'; import { api } from '../../api/client';
import { AuthError } from '../../api/client';
const devices = ref([] as any[]), buildings = ref([] as any[]), selBuilding = ref(0), selBuildingName = ref(''), buildingNames = ref([] as string[]), loading = ref(true);
onMounted(async () => {
  try {
    const b = await api.get('/buildings'); buildings.value = b.data as any[];
    buildingNames.value = (b.data as any[]).map((b:any) => b.name);
    buildingNames.value.unshift('全部楼宇');
  } catch (e) { if (!(e instanceof AuthError)) uni.showToast({ title: '楼宇加载失败', icon: 'none' }) }
  await load();
  loading.value = false;
});
const load = async () => {
  try {
    const path = selBuilding.value > 0 ? '/devices?building_id='+selBuilding.value : '/devices';
    const r = await api.get(path); devices.value = (r.data as any) || [];
  } catch (e) { if (!(e instanceof AuthError)) uni.showToast({ title: '设备加载失败', icon: 'none' }) }
};
const onBldChange = (e: any) => {
  const i = e.detail.value as number;
  if(i===0){selBuilding.value=0;selBuildingName.value='';}else{selBuilding.value=buildings.value[i-1]?.id;selBuildingName.value=buildingNames.value[i];}
  load();
};
const goDetail = (d: any) => { if (!d?.id) return; uni.navigateTo({ url: '/pages/detail/detail?id='+encodeURIComponent(d.id) }) }
const goHistory = (d: any) => { if (!d?.id) return; uni.navigateTo({ url: '/pages/history/history?device_id='+d.id+'&device_name='+encodeURIComponent(d.name) }) }
</script>
<style>
.list { padding: 20rpx; }
.loading { text-align: center; padding: 80rpx; color: #999; font-size: 28rpx; }
.empty { text-align: center; padding: 80rpx; color: #999; font-size: 28rpx; }
.filter-bar { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16rpx; }
.picker { background: #006875; color: #fff; padding: 12rpx 24rpx; border-radius: 8rpx; font-size: 26rpx; }
.count { font-size: 24rpx; color: #999; }
.item { display: flex; justify-content: space-between; align-items: center; background: #fff; padding: 24rpx; margin-bottom: 12rpx; border-radius: 12rpx; }
.name { font-size: 30rpx; font-weight: 600; display: block; }
.type { font-size: 24rpx; color: #999; }
.status { font-size: 24rpx; } .on { color: #52c41a; } .off { color: #999; }
</style>
