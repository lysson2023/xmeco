import js from '@eslint/js'
import globals from 'globals'
import reactHooks from 'eslint-plugin-react-hooks'
import reactRefresh from 'eslint-plugin-react-refresh'
import tseslint from 'typescript-eslint'
import { defineConfig, globalIgnores } from 'eslint/config'

export default defineConfig([
  globalIgnores(['dist']),
  {
    files: ['**/*.{ts,tsx}'],
    extends: [
      js.configs.recommended,
      tseslint.configs.recommended,
      reactHooks.configs.flat.recommended,
      reactRefresh.configs.vite,
    ],
    languageOptions: {
      globals: globals.browser,
    },
    rules: {
      // TODO: 当前类型定义债务较重，先降级为 warning，后续逐步重构为具体类型
      '@typescript-eslint/no-explicit-any': 'warn',
      // 将实验性 hooks 规则降级，避免对现有 fetch-in-effect 模式过度报错
      'react-hooks/no-set-state-in-effect': 'warn',
      'react-hooks/no-set-state-in-layout-effect': 'warn',
    },
  },
])
