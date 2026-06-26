<template><view class="detail">
  <view class="head"><text class="name">{{dev.name}}</text><text class="type">{{dev.device_type}}</text></view>
  <view class="props" v-for="p in props" :key="p.id">
    <text class="pn">{{p.prop_name}}</text><text class="pv">{{p.prop_value}} {{p.unit}}</text>
  </view>
  <view class="controls">
    <button class="ctrl on" :disabled="controlling" @click="confirmControl('start')">{{ controlling ? '发送中...' : '启动' }}</button>
    <button class="ctrl off" :disabled="controlling" @click="confirmControl('stop')">{{ controlling ? '发送中...' : '停止' }}</button>
  </view>
  <!-- 确认弹窗 -->
  <view class="modal-mask" v-if="showConfirm" @click="showConfirm=false">
    <view class="modal-box" @click.stop>
      <text class="modal-title">确认操作</text>
      <text class="modal-body">确定要{{confirmAction==='start'?'启动':'停止'}}设备 "{{dev.name}}" 吗？</text>
      <view class="modal-btns">
        <button class="modal-btn cancel" @click="showConfirm=false">取消</button>
        <button class="modal-btn ok" :loading="controlling" @click="doControl">{{ controlling ? '执行中...' : '确定' }}</button>
      </view>
    </view>
  </view>
</view></template>
<script setup lang="ts">
import { ref, onMounted } from 'vue'; import { api } from '../../api/client';
import { AuthError } from '../../api/client';
const dev = ref({} as any); const props = ref([] as any[])
const controlling = ref(false)
const showConfirm = ref(false)
const confirmAction = ref('')
onMounted(async () => {
  const pages = getCurrentPages(); const id = (pages[pages.length-1] as any).options?.id
  if (!id) { uni.showToast({ title: '设备ID无效', icon: 'none' }); return }
  try {
    const [d, p] = await Promise.all([api.get('/devices/'+id), api.get('/properties?device_id='+id)])
    dev.value = d.data as any; props.value = (p.data as any) || []
  } catch (e) { if (!(e instanceof AuthError)) uni.showToast({ title: '加载失败', icon: 'none' }) }
})
const confirmControl = (a: string) => {
  confirmAction.value = a
  showConfirm.value = true
}
const doControl = async () => {
  controlling.value = true
  try {
    await api.post('/devices/'+dev.value.id+'/control', { action: confirmAction.value })
    uni.showToast({ title: '已发送', icon: 'success' })
    showConfirm.value = false
  } catch (e: any) {
    uni.showToast({ title: e?.message || '发送失败', icon: 'none' })
  } finally {
    controlling.value = false
  }
}
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
.ctrl[disabled] { opacity: 0.5; }
.modal-mask { position: fixed; inset: 0; background: rgba(0,0,0,0.5); display: flex; align-items: center; justify-content: center; z-index: 999; }
.modal-box { background: #fff; border-radius: 16rpx; padding: 40rpx; width: 80%; max-width: 560rpx; }
.modal-title { font-size: 32rpx; font-weight: 600; display: block; margin-bottom: 16rpx; }
.modal-body { font-size: 28rpx; color: #666; display: block; margin-bottom: 30rpx; }
.modal-btns { display: flex; gap: 20rpx; justify-content: flex-end; }
.modal-btn { font-size: 28rpx; padding: 16rpx 40rpx; border-radius: 10rpx; border: none; }
.cancel { background: #f0f0f0; color: #666; } .ok { background: #006875; color: #fff; }
</style>
