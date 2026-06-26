# XMECO Web 大屏设计规范 (Design System)

> 版本: v1.0  
> 适用范围: Web 管理后台大屏 (`screen-src/`) + 未来所有数据可视化页面  
> 技术约束: React 19 + Ant Design 6 + TypeScript + Vite  
> 配色基准: 青绿科技风 (Cyan-Teal Sci-Fi)

---

## 1. 设计理念

### 1.1 三个核心原则

| 原则 | 描述 |
|------|------|
| **深色基底，发光前景** | 所有页面背景使用深色 (`#040d0d` ~ `#0b1515`)，数据/控件使用青绿色 (`#00daf3` ~ `#006666`) 发光强调 |
| **玻璃质感，层次分明** | 卡片/面板使用半透明背景 + backdrop-blur，通过边框发光和阴影区分活跃/非活跃状态 |
| **工业克制，功能优先** | 动画仅用于状态指示（在线/离线/告警），不过度装饰；字体统一三种家族各司其职 |

### 1.2 不适用的场景

- **管理后台 CRUD 页面**（`/projects`、`/users` 等）继续使用 Ant Design 默认主题，不需要此设计规范
- **小程序端**不受此规范约束
- **登录页**可使用此规范的减弱版本（少动画，聚焦表单）

---

## 2. 色彩系统

### 2.1 语义色板

所有颜色定义在 Tailwind `theme.extend.colors` 中，参考 Material Design 3 的 token 命名。以下色值**直接从参考实现提取，不可随意变更**。

#### 背景色阶

| Token | 色值 | 用途 |
|-------|------|------|
| `atmospheric-surface` | `#040d0d` | 页面最深底色（登录页） |
| `background` | `#0b1515` | 页面主背景 |
| `surface` | `#0b1515` | 卡片/面板背景 |
| `surface-dim` | `#0b1515` | 略暗的面板背景 |
| `surface-container-lowest` | `#061010` | 最低层级容器 |
| `surface-container-low` | `#131d1d` | 低层级容器 |
| `surface-container` | `#172121` | 默认容器背景 |
| `surface-container-high` | `#212c2c` | 高层级容器 |
| `surface-container-highest` | `#2c3737` | 最高层级容器 |
| `surface-bright` | `#313b3b` | 高亮表面 |
| `inverse-surface` | `#d9e5e4` | 反色表面（浅色模式下） |

#### 主色（Primary — 用于主要操作按钮、选中态）

| Token | 色值 | 用途 |
|-------|------|------|
| `primary` | `#95d1d0` | 主色文字/图标 |
| `primary-container` | `#004c4c` | 主色容器背景 |
| `on-primary` | `#003737` | 主色上的文字 |
| `on-primary-container` | `#7fbbba` | 主色容器上的文字 |
| `primary-fixed` | `#b1eeed` | 固定主色（浅色模式） |
| `primary-fixed-dim` | `#95d1d0` | 固定主色暗调 |
| `inverse-primary` | `#2a6767` | 反色主色 |

#### 辅色（Secondary — 用于激活/在线状态、数据高亮）

| Token | 色值 | 用途 |
|-------|------|------|
| `secondary` | `#9cefff` | 辅色文字（在线状态） |
| `secondary-container` | `#01daf3` | **核心发光色** — 边框发光、活跃设备、动画高亮 |
| `secondary-fixed` | `#9cefff` | 固定辅色 |
| `secondary-fixed-dim` | `#01daf3` | 固定辅色暗调 |
| `on-secondary` | `#00363d` | 辅色上的文字 |
| `on-secondary-container` | `#005b67` | 辅色容器上的文字 |

#### 第三色（Tertiary — 用于次要数据、信息状态）

| Token | 色值 | 用途 |
|-------|------|------|
| `tertiary` | `#84d4d3` | 第三色文字 |
| `tertiary-container` | `#004c4c` | 第三色容器 |
| `tertiary-fixed` | `#a0f0f0` | 固定第三色 |
| `tertiary-fixed-dim` | `#84d4d3` | 固定第三色暗调 |

#### 功能色

| Token | 色值 | 用途 |
|-------|------|------|
| `error` | `#ffb4ab` | 告警/故障状态 |
| `error-container` | `#93000a` | 告警背景 |
| `on-error` | `#690005` | 告警文字 |
| `on-error-container` | `#ffdad6` | 告警容器文字 |
| `glow-cyan` | `rgba(0, 218, 243, 0.4)` | 全局发光色（阴影用） |
| `glow-green` | `rgba(127, 255, 212, 0.6)` | 节能/成功发光（`#7fffd4`） |
| `glow-blue` | `rgba(0, 162, 255, 0.6)` | 冷冻/次要设备发光（`#00a2ff`） |

#### 文本色 & 边框

| Token | 色值 | 用途 |
|-------|------|------|
| `on-surface` | `#d9e5e4` | 主文字色 |
| `on-surface-variant` | `#bfc8c8` | 次要文字色 |
| `on-background` | `#d9e5e4` | 背景上文字 |
| `outline` | `#899392` | 组件边框 |
| `outline-variant` | `#3f4848` | 卡片边框 |
| `surface-variant` | `#2c3737` | 表面变体 |

#### 特殊效果色

| Token | 色值 | 用途 |
|-------|------|------|
| `glass-fill-dark` | `rgba(0, 0, 0, 0.60)` | 深色玻璃面板填充 |
| `glass-fill-light` | `rgba(255, 255, 255, 0.05)` | 浅色玻璃面板填充 |
| `deep-teal` | `#006666` | 深度青色（作为背景渐变中间色） |

### 2.2 配色使用规则

| 规则 | 说明 |
|------|------|
| **活跃/在线** | 必须使用 `secondary` / `secondary-container`（`#01daf3`）系列，搭配发光阴影 |
| **待机/离线** | 使用 `white/50` 透明度 + `opacity-50`，不发光 |
| **告警/故障** | 使用 `error`（`#ffb4ab`）+ `glow-pulse` 红色变体 |
| **节能/正向指标** | 使用 `glow-green`（`#7fffd4`）系列 |
| **冷冻系统** | 使用 `glow-blue`（`#00a2ff`）系列，与冷却塔青色区分 |
| **文字层次** | 主文字 `on-surface` → 次要 `on-surface-variant` → 禁用 `white/30` |
| **禁止** | 不使用纯白 `#ffffff` 作为主文字色，统一用 `#d9e5e4` |

---

## 3. 字体系统 (Typography)

### 3.1 字体家族

| Token | 字体栈 | 用途 |
|-------|--------|------|
| `headline` / `display-hero` / `headline-lg` | `'Space Grotesk', sans-serif` | 标题、大数字、品牌名称 |
| `body` / `title-lg` / `body-md` | `'Manrope', sans-serif` | 正文、表格、描述 |
| `label` / `label-md` / `label-sm-caps` | `'Inter', sans-serif` | 标签、按钮文字、导航、小字 |

**加载方式**：通过 Google Fonts CDN 或本地 `@fontsource` 包引入，不要使用 `@import`。

### 3.2 字号阶梯

| Token | Size | Line Height | Weight | Letter Spacing | 用途 |
|-------|------|-------------|--------|---------------|------|
| `display-hero` | 56px | 64px | 700 | -0.02em | 首页核心指标（节能率、总节电量） |
| `headline-lg` | 32px | 40px | 600 | — | 页面主标题、面板标题 |
| `headline-md` | 24px | 32px | 500 | — | 卡片标题、设备名称 |
| `title-lg` | 20px | 28px | 600 | — | 区块标题 |
| `body-lg` | 18px | 28px | 400 | — | 强调正文 |
| `body-md` | 16px | 24px | 400 | — | 默认正文 |
| `label-md` | 14px | 20px | 500 | 0.02em | 导航项、表单标签 |
| `label-sm-caps` | 12px | 16px | 700 | 0.05em | 小字标签（全大写跟踪） |
| 数据小数 | 12-14px | — | — | — | 跟在 `display-hero` 后的单位（kWh、°C、%） |

### 3.3 字体使用规则

| 规则 | 说明 |
|------|------|
| **数字** | 任何数据指标（温度、功率、百分比）使用 `font-headline`（Space Grotesk） |
| **标签** | 任何标题/标签文字使用 `font-label`（Inter），nav 和 label 场合 |
| **正文** | 任何描述性文字使用 `font-body`（Manrope） |
| **混排** | 大数字 + 小单位 = `display-hero` + `title-lg`，如 `<span class="font-display-hero">24<span class="font-title-lg text-secondary mt-2">°C</span></span>` |

---

## 4. 间距与布局

### 4.1 间距 Token

| Token | 值 | 用途 |
|-------|-----|------|
| `grid-minor` | 5px | 细微间距（图标与文字之间） |
| `grid-major` | 20px | 卡片内间距、网格单位 |
| `margin-mobile` | 16px | 移动端外边距 |
| `margin-desktop` | 48px | 桌面端外边距（大屏左右留白） |
| `container-gap` | 32px | 左右两栏之间的间距 |
| `gutter` | 24px | 卡片之间间距 |

### 4.2 布局规则

| 规则 | 说明 |
|------|------|
| **大屏最小宽度** | 1280px。低于此宽度不展示，不使用响应式 |
| **左右留白** | 统一 `px-margin-desktop`（48px） |
| **顶部导航高度** | 固定 `h-20`（80px），包含品牌名 + 项目/建筑选择器 + 用户信息 |
| **二级导航高度** | 固定 `h-14`（56px），水平 Tab 导航 |
| **主内容区** | `flex-1 overflow-auto`，弹性占满剩余高度 |
| **左侧栏宽度** | 固定 `w-48`（192px），放天气卡片 + 系统日志 |
| **右侧栏宽度** | 固定 `w-40`（160px），放节能指标卡片 |
| **中央区域** | `flex-1`，设备拓扑图，`min-w-[600px]` |

---

## 5. 组件样式

### 5.1 玻璃面板 (Glass Panel)

核心组件。所有卡片、面板、下拉菜单必须使用此样式。

```css
.glass-panel {
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.08) 0%, rgba(255, 255, 255, 0.02) 100%);
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  border: 1px solid rgba(1, 218, 243, 0.25);
  box-shadow: 0 8px 32px 0 rgba(0, 0, 0, 0.2), inset 0 0 20px rgba(1, 218, 243, 0.05);
}
```

**变体**：
- `glass-fill-dark`：`bg-black/60` + `backdrop-blur-xl`（用于顶部导航栏、底部 footer）
- `glass-fill-light`：`bg-white/5`（用于表单输入框）

### 5.2 设备状态指示

| 状态 | 视觉效果 | Tailwind 组合 |
|------|---------|--------------|
| **运行中/在线** | 青绿发光边框 + 阴影 + 浮动动画 | `border-secondary/60 glow-pulse animate-float shadow-[0_0_30px_rgba(1,218,243,0.3)]` |
| **待机/离线** | 半透明灰色 + 无发光 | `opacity-50` + 无 `glow-pulse` + 无 `shadow` |
| **故障/告警** | 红色发光 | `border-error shadow-[0_0_8px_rgba(255,180,171,0.8)]` |

设备节点结构（以冷却塔为例）：
```
圆形/圆角容器 (w-24 h-24 / w-32 h-20 rounded-xl / rounded-full)
  ├── Material Symbol 图标
  └── 设备名称 (font-label-sm-caps)
```

### 5.3 发光脉冲动画 (Glow Pulse)

```css
@keyframes pulse-glow {
  0%   { box-shadow: 0 0 10px rgba(1, 218, 243, 0.3), inset 0 0 10px rgba(1, 218, 243, 0.1); border-color: rgba(1, 218, 243, 0.4); }
  50%  { box-shadow: 0 0 30px rgba(1, 218, 243, 0.8), inset 0 0 20px rgba(1, 218, 243, 0.4); border-color: rgba(1, 218, 243, 0.9); }
  100% { box-shadow: 0 0 10px rgba(1, 218, 243, 0.3), inset 0 0 10px rgba(1, 218, 243, 0.1); border-color: rgba(1, 218, 243, 0.4); }
}
.glow-pulse { animation: pulse-glow 2.5s infinite ease-in-out; }
```

**变体颜色**：
- 默认青色：`rgba(1, 218, 243, ...)`
- 绿色（节能）：`rgba(127, 255, 212, ...)`
- 蓝色（冷冻系统）：`rgba(0, 162, 255, ...)`
- 红色（故障）：`rgba(255, 180, 171, ...)`

### 5.4 流动线 (Flow Lines)

用于设备拓扑图中的连接线。使用 SVG `<path>` + CSS `stroke-dasharray` 动画。

```css
.flow-line {
  stroke-dasharray: 10;
  animation: flow 1s linear infinite;
  filter: drop-shadow(0 0 6px rgba(1, 218, 243, 0.9));
}
@keyframes flow {
  to { stroke-dashoffset: -20; }
}
```

**线型**：
- 实线：活跃连接（`stroke="#00f2ff" stroke-width="3"`）
- 虚线：备用/非活跃连接（`stroke-dasharray="4 4" stroke-width="1.5" opacity-40`）

### 5.5 浮动动画 (Float)

```css
@keyframes float {
  0%   { transform: translateY(0px); }
  50%  { transform: translateY(-6px); }
  100% { transform: translateY(0px); }
}
.animate-float { animation: float 4s ease-in-out infinite; }
.delay-1 { animation-delay: 0.5s; }
.delay-2 { animation-delay: 1.0s; }
.delay-3 { animation-delay: 1.5s; }
```

仅用于活跃设备节点，离线设备不使用。

### 5.6 按钮

| 类型 | 样式 | 使用场景 |
|------|------|---------|
| **主按钮 (CTA)** | `bg-gradient-to-r from-cyan-600 to-cyan-400 hover:... text-white font-bold rounded-xl shadow-[0_0_30px_rgba(0,218,243,0.4)]` | 登录、提交、确认操作 |
| **导航项 (Active)** | `bg-secondary/10 text-secondary border-b-2 border-secondary` | 当前选中的 Tab |
| **导航项 (Inactive)** | `text-white/70 hover:bg-secondary/5 hover:text-secondary` | 未选中的 Tab |
| **幽灵按钮** | `glass-panel border-secondary/30 hover:border-secondary/60 hover:shadow-[0_0_15px]` | 项目选择器、用户菜单 |

---

## 6. 动画与效果

### 6.1 动画总则

| 规则 | 说明 |
|------|------|
| **仅状态动画** | 动画只用于表示设备在线/运行/告警，不在纯装饰元素上使用复杂动画 |
| **持续运行** | 核心动画（glow-pulse、flow-line、float）`infinite`，不停止 |
| **性能** | 动画属性限制为 `transform`、`opacity`、`box-shadow`、`filter`，避免 `width/height` 动画 |
| **GPU 加速** | 使用 `will-change: transform, opacity` 或 Tailwind 的 `transform-gpu` |
| **禁止** | 不使用 `animation: spin` 做大尺寸元素旋转（参考实现中仅边框轨道使用 120s 慢旋转） |

### 6.2 动画清单

| 动画名 | 用途 | 时长 | 缓动 |
|--------|------|------|------|
| `pulse-glow` | 设备在线/运行指示 | 2.5s | ease-in-out |
| `flow` | 连接线数据流 | 1s | linear |
| `float` | 设备节点浮动 | 4s | ease-in-out |
| `scan` | 扫描线效果（登录页） | 3s | cubic-bezier |
| `fiberMove` | 光纤线（登录页） | 4s | ease-in-out |
| `flicker` | 接点闪烁（登录页） | 2s | ease |
| 轨道旋转 | 背景装饰环（登录页） | 120s / 180s | linear |

### 6.3 背景系统

大屏背景由两层构成：

**第一层：WebGL / Canvas（底层）**
- 使用 simplex noise shader（参考实现中 `glcanvas`）
- 颜色：深青绿色噪声纹理 (`#006666` base)
- 鼠标交互：subtle glow 跟随
- 渲染分辨率：固定 1280x720 或视口分辨率

**第二层：CSS 网格（叠加层）**
```css
.grid-bg {
  background-size: 20px 20px;
  background-image:
    linear-gradient(to right, rgba(1, 218, 243, 0.05) 1px, transparent 1px),
    linear-gradient(to bottom, rgba(1, 218, 243, 0.05) 1px, transparent 1px);
}
```

> **简化方案**：如果 WebGL 集成成本过高（需要引入 twgl.js），可以暂时只用 CSS 网格背景 + 径向渐变光晕，效果已足够 80% 的视觉需求。WebGL 作为 P2 优化项。

---

## 7. 图标系统

### 7.1 图标规范

| 规则 | 说明 |
|------|------|
| **图标库** | `@ant-design/icons`（当前项目已安装），用于标准 UI 图标 |
| **设备图标** | 优先使用 `@ant-design/icons` 中的语义图标：`HeatMapOutlined`（热泵）、`DashboardOutlined`（监控）、`ThunderboltOutlined`（电力） |
| **Material Symbols** | 参考实现使用的 Material Symbols 不作为硬性要求。如需引入，使用 `material-symbols-outlined` 字体类 |
| **图标大小** | 导航：`text-[18px]`；卡片内：`text-sm` / `text-xl`；设备节点：`text-2xl` / `text-3xl` |
| **发光** | 活跃状态图标添加 `drop-shadow-[0_0_8px_rgba(1,218,243,0.8)]` |

### 7.2 设备图标映射

| 设备类型 | Ant Design Icon | Material Symbol (备选) |
|----------|----------------|----------------------|
| 主机/冷水机组 | — | `ac_unit` |
| 冷却塔 | — | `mode_fan` |
| 冷却泵/冷冻泵 | `HeatMapOutlined` | `heat_pump` |
| 阀门 | `NodeIndexOutlined` | `valve` |
| 电表 | `ThunderboltOutlined` | `bolt` |

---

## 8. 响应式与适配

| 规则 | 说明 |
|------|------|
| **仅桌面端** | 大屏仅支持 ≥1280px 宽度，不实现移动端布局 |
| **全屏** | 大屏建议以 `100vw × 100vh` 全屏展示，无浏览器 chrome |
| **字体缩放** | 使用 Tailwind 的 `text-[size]` 固定像素值，不随 viewport 缩放 |
| **高清屏** | 背景网格和 WebGL 按 `devicePixelRatio` 渲染 |

---

## 9. 与现有代码的衔接

### 9.1 当前状态

| 项 | 现状 |
|----|------|
| CSS 框架 | 未引入 Tailwind（当前仅用 Ant Design 内置样式 + 一个 `Login.css`） |
| 大屏页面 | `web/admin/src/pages/Screen.tsx`，使用 Ant Design 组件 + 内联 style |
| 构建 | Vite，无 PostCSS 配置 |

### 9.2 接入步骤（建议）

```
Phase 1: 安装 Tailwind
  1. npm install -D tailwindcss @tailwindcss/vite
  2. 在 vite.config.ts 中添加 tailwindcss 插件
  3. 创建 src/index.css 引入 @tailwind base/components/utilities
  4. 创建 tailwind.config.ts，粘贴本文档第 2 节的 colors + 第 3 节的 fontFamily + 第 4 节的 spacing

Phase 2: 改造 Screen.tsx
  1. 将内联 style 替换为 Tailwind class
  2. 顶部导航 → glass-fill-dark + flex
  3. 设备拓扑 → glass-panel + flow-line SVG
  4. 指标卡片 → glass-panel + glow-pulse 变体

Phase 3: 统一 Login 页
  1. 参考第一段 HTML 的登录页风格
  2. 替换 Login.css 为 Tailwind class
```

### 9.3 大屏入口

大屏通过 `screen-src/` 独立入口构建（端口 3001），与后台管理分开。大屏的 Tailwind 配置应与此规范完全一致。

---

## 10. 禁止事项

| ❌ 禁止 | 原因 |
|---------|------|
| 使用 Ant Design 的浅色主题 | 与深色科技风冲突 |
| 使用 `#ffffff` 纯白作为文字色 | 破坏整体调性，应用 `#d9e5e4` |
| 在非状态元素上使用 `animate-pulse` / `animate-spin` | 干扰数据阅读 |
| 使用圆角 > `xl`（12px）的卡片 | 工业风格保持直角或小圆角 |
| 混合使用多种字体家族外的字体 | 破坏视觉一致性 |
| 在设备拓扑图中使用 `<img>` 代替 SVG | 无法做流线动画 |
| 在大屏中使用 Ant Design Modal 弹窗 | 应用 glass-panel 自定义弹窗 |
