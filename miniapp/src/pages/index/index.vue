<template><view class="home">
  <view class="header"><text class="greet">XMECO</text><text class="sub">{{userName}} · Energy Monitor</text></view>

  <!-- 天气卡片 -->
  <view class="weather-card" v-if="weather">
    <view class="weather-main">
      <text class="weather-city">📍 {{weather.city_name}}</text>
      <view class="weather-temp-row">
        <text class="weather-temp">{{weather.temp}}°</text>
        <text class="weather-text">{{weather.weather_text}}</text>
      </view>
    </view>
    <view class="weather-details">
      <view class="wd-item"><text class="wd-lbl">体感</text><text class="wd-val">{{weather.feels_like}}°</text></view>
      <view class="wd-item"><text class="wd-lbl">湿度</text><text class="wd-val">{{weather.humidity}}%</text></view>
      <view class="wd-item"><text class="wd-lbl">风向</text><text class="wd-val">{{weather.wind_dir}} {{weather.wind_scale}}级</text></view>
    </view>
  </view>

  <view class="cards">
    <view class="card"><text class="val green">{{cfg.online_devices||'0'}}</text><text class="lbl">在线设备</text></view>
    <view class="card"><text class="val red">{{cfg.today_alarms||'0'}}</text><text class="lbl">今日告警</text></view>
    <view class="card"><text class="val cyan">{{cfg.running_days||'0'}}天</text><text class="lbl">运行天数</text></view>
    <view class="card"><text class="val">{{cfg.power_saved||'0'}}度</text><text class="lbl">累计节电</text></view>
  </view>
  <view class="title">快捷操作</view>
  <view class="actions">
    <view class="btn" @click="goDevices"><text>设备管理</text></view>
    <view class="btn" @click="goAlarms"><text>告警中心</text></view>
    <view class="btn" @click="goHistory"><text>历史数据</text></view>
  </view>
</view></template>
<script setup lang="ts">
import { ref, onMounted } from 'vue';
import { api } from '../../api/client';
import { AuthError } from '../../api/client';
const cfg = ref<any>({});
const userName = ref('');
const weather = ref<any>(null);
const loading = ref(true);
onMounted(async () => {
  // 无 token 时直接跳过，等待 App.vue 的 reLaunch 生效，避免无效请求和 401 竞争
  if (!api.getToken()) return
  try { const r = await api.get('/dashboard'); cfg.value = (r.data || {}) as any; } catch (e) { if (!(e instanceof AuthError)) uni.showToast({ title: '仪表盘数据加载失败', icon: 'none' }) }
  try { const r = await api.get('/auth/me'); userName.value = (r.data as any)?.username || ''; } catch (e) { if (!(e instanceof AuthError)) uni.showToast({ title: '用户信息加载失败', icon: 'none' }) }
  // 获取项目天气
  try {
    const r = await api.get('/projects');
    const projects = r.data as any[];
    if (projects && projects.length > 0 && projects[0]?.city_id) {
      const w = await api.get(`/weather/project?project_id=${encodeURIComponent(projects[0].id)}`);
      weather.value = w.data as any;
    }
  } catch { /* 天气非关键数据，静默 */ }
  loading.value = false;
});
const goDevices = () => uni.switchTab({ url: '/pages/devices/devices' })
const goAlarms = () => uni.switchTab({ url: '/pages/alarms/alarms' })
const goHistory = () => uni.navigateTo({ url: '/pages/history/history' })
</script>
<style>
.home { padding: 20rpx; background: #f5f7fa; min-height: 100vh; }
.header { padding: 30rpx 0; text-align: center; }
.greet { font-size: 44rpx; font-weight: 700; color: #006875; display: block; }
.sub { font-size: 26rpx; color: #999; margin-top: 8rpx; }
/* 天气卡片 */
.weather-card { background: linear-gradient(135deg, #667eea, #764ba2); border-radius: 20rpx; padding: 30rpx; margin: 16rpx 0; color: #fff; }
.weather-main { margin-bottom: 20rpx; }
.weather-city { font-size: 24rpx; opacity: 0.85; }
.weather-temp-row { display: flex; align-items: baseline; gap: 16rpx; margin-top: 8rpx; }
.weather-temp { font-size: 64rpx; font-weight: 700; }
.weather-text { font-size: 28rpx; opacity: 0.9; }
.weather-details { display: flex; gap: 40rpx; }
.wd-item { display: flex; flex-direction: column; }
.wd-lbl { font-size: 22rpx; opacity: 0.7; }
.wd-val { font-size: 26rpx; margin-top: 4rpx; }
.cards { display: flex; flex-wrap: wrap; gap: 16rpx; margin: 20rpx 0; }
.card { flex: 1; min-width: 40%; background: #fff; border-radius: 16rpx; padding: 30rpx; text-align: center; box-shadow: 0 2rpx 12rpx rgba(0,0,0,0.06); }
.val { font-size: 48rpx; font-weight: 700; display: block; }
.lbl { font-size: 24rpx; color: #999; margin-top: 8rpx; }
.green { color: #52c41a; } .red { color: #ff4d4f; } .cyan { color: #006875; }
.title { font-size: 32rpx; font-weight: 600; margin: 30rpx 0 16rpx; }
.actions { display: flex; gap: 16rpx; }
.btn { flex: 1; background: #006875; color: #fff; text-align: center; padding: 24rpx; border-radius: 12rpx; font-size: 28rpx; }
</style>
