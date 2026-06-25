<template><view class="page">
  <!-- 设备选择 -->
  <view class="row">
    <picker mode="selector" :range="devNames" @change="onDevChange"><view class="pick">{{selDevName||'选择设备'}}</view></picker>
  </view>

  <!-- 指标选择 -->
  <view class="row">
    <picker mode="selector" :range="metricNames" @change="onMetricChange" v-if="metrics.length>0"><view class="pick pick-sm">{{selMetric||'全部指标'}}</view></picker>
    <picker mode="selector" :range="intervalNames" @change="onIntervalChange"><view class="pick pick-sm">{{selInterval||'原始'}}</view></picker>
  </view>

  <!-- 快捷日期 -->
  <view class="quick-row">
    <text class="tag" :style="quick==i?activeTag:{}" v-for="(d,i) in quickDates" :key="i" @click="setQuick(i)">{{d.label}}</text>
  </view>

  <!-- 自定义日期 -->
  <view class="row date-row">
    <picker mode="date" :end="todayDate" @change="onStart"><view class="pick pick-sm">{{startDate||'开始日期'}}</view></picker>
    <text style="margin:0 8rpx;color:#999">—</text>
    <picker mode="date" :end="todayDate" @change="onEnd"><view class="pick pick-sm">{{endDate||'结束日期'}}</view></picker>
  </view>

  <!-- 数据表格 -->
  <view v-if="loading" class="loading">加载中...</view>
  <view v-else-if="list.length===0" class="empty">暂无数据</view>
  <view v-else class="table">
    <view class="th"><text class="td" style="flex:3">时间</text><text class="td" style="flex:2">值</text></view>
    <view class="tr" v-for="(r,i) in list" :key="i">
      <text class="td" style="flex:3;color:#666">{{fmtTs(r.ts)}}</text>
      <text class="td" style="flex:2;font-weight:600;color:#006875">{{r.value!=null?fmtVal(r.value):(r.avg!=null?fmtVal(r.avg):'-')}}</text>
    </view>
  </view>
</view></template>

<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { api } from '../../api/client';

const allDevices = ref<any[]>([]);
const metrics = ref<any[]>([]);
const devNames = ref<string[]>([]);
const metricNames = ref<string[]>([]);
const selDev = ref(0); const selDevName = ref('');
const selMetric = ref(''); const selInterval = ref('raw');
const intervalNames = ['原始','分钟','小时','天','周','月','年'];
const quick = ref(0);
const quickDates = [{label:'今天'},{label:'本周'},{label:'本月'},{label:'自定义'}];
const startDate = ref(''); const endDate = ref('');
const todayDate = new Date().toISOString().slice(0,10);
const list = ref<any[]>([]);
const loading = ref(false);

const activeTag = `background:#006875;color:#fff;border-color:#006875;`;

onMounted(async () => {
  try { const r = await api.get('/devices'); allDevices.value = r.data as any[]; devNames.value = (r.data as any[]).map((d:any)=>d.name); } catch {}
  // Pre-select device from URL params
  const pages = getCurrentPages();
  const opts = (pages[pages.length-1] as any).options;
  if (opts?.device_id) {
    const idx = allDevices.value.findIndex((d:any) => Number(d.id)===Number(opts.device_id));
    if (idx>=0) { selDev.value = allDevices.value[idx].id; selDevName.value = devNames.value[idx]; loadMetrics(); }
  }
  setQuick(0);
});

function fmtDate(d: Date) { return d.toISOString().slice(0,10); }
function fmtTs(v: string) { return v ? v.slice(0,16).replace('T',' ') : '-'; }
function fmtVal(v: number) { return Number(v).toFixed(1); }

function setQuick(i: number) {
  quick.value = i; const now = new Date();
  if (i===0) { startDate.value = fmtDate(now); endDate.value = fmtDate(now); }
  else if (i===1) { const wk = new Date(now); wk.setDate(now.getDate()-7); startDate.value = fmtDate(wk); endDate.value = fmtDate(now); }
  else if (i===2) { startDate.value = fmtDate(new Date(now.getFullYear(),now.getMonth(),1)); endDate.value = fmtDate(now); }
  load();
}

function onDevChange(e: any) { const i = e.detail.value as number; selDev.value = allDevices.value[i]?.id; selDevName.value = devNames.value[i]; loadMetrics(); }
function onMetricChange(e: any) { selMetric.value = metricNames.value[e.detail.value]; if(selMetric.value==='全部指标')selMetric.value=''; load(); }
function onIntervalChange(e: any) { selInterval.value = ['raw','minute','hour','day','week','month','year'][e.detail.value]; load(); }
function onStart(e: any) { startDate.value = e.detail.value; quick.value = 3; load(); }
function onEnd(e: any) { endDate.value = e.detail.value; quick.value = 3; load(); }

async function loadMetrics() {
  if (!selDev.value) return;
  try { const r = await api.get('/properties?device_id='+selDev.value); metrics.value = r.data as any[]; metricNames.value = ['全部指标',...((r.data as any[]).map((p:any)=>p.prop_name))]; } catch {}
  load();
}

async function load() {
  if (!selDev.value || !startDate.value || !endDate.value) return;
  loading.value = true;
  try {
    const params = `?device_id=${selDev.value}&start=${startDate.value}&end=${endDate.value}&interval=${selInterval.value}${selMetric.value?'&metric='+selMetric.value:''}`;
    const r = await api.get('/logs/telemetry'+params);
    const data = r.data as any[];
    list.value = (data||[]).slice(0,200);
  } catch { list.value = []; }
  loading.value = false;
}
</script>

<style>
.page { padding: 20rpx; background: #f5f7fa; min-height: 100vh; }
.row { display: flex; gap: 16rpx; margin-bottom: 12rpx; }
.quick-row { display: flex; gap: 12rpx; margin-bottom: 12rpx; }
.tag { padding: 8rpx 20rpx; border: 1rpx solid #d9d9d9; border-radius: 20rpx; font-size: 24rpx; color: #666; }
.pick { background: #006875; color: #fff; padding: 14rpx 28rpx; border-radius: 10rpx; font-size: 26rpx; }
.pick-sm { font-size: 24rpx; padding: 10rpx 20rpx; }
.date-row { display: flex; align-items: center; margin-bottom: 16rpx; }
.table { background: #fff; border-radius: 12rpx; overflow: hidden; }
.th { display: flex; background: #f0f0f0; padding: 16rpx 24rpx; font-size: 24rpx; color: #666; font-weight: 600; }
.tr { display: flex; padding: 14rpx 24rpx; border-bottom: 1rpx solid #f0f0f0; }
.td { font-size: 24rpx; }
.loading,.empty { text-align: center; padding: 100rpx; color: #999; font-size: 28rpx; }
</style>
