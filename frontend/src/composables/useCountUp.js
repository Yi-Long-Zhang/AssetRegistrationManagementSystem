import { ref, watch, onBeforeUnmount } from 'vue'

/**
 * 数字滚动（零依赖，requestAnimationFrame 实现）
 * 用于看板统计卡片的数值动画，配合 prefers-reduced-motion 无障碍降级。
 *
 * @param {import('vue').Ref<number> | (() => number)} source 目标值：ref 或 getter 函数
 * @param {object} [options]
 * @param {number} [options.duration=800] 动画时长（毫秒）
 * @param {number} [options.decimals=0] 小数位
 * @param {(t:number)=>number} [options.easing=easeOutCubic] 缓动函数
 * @returns {import('vue').Ref<number>} 动画过程中的显示值
 */
export function useCountUp(source, options = {}) {
  const { duration = 800, decimals = 0, easing = easeOutCubic } = options

  const display = ref(0)
  let rafId = null

  function prefersReducedMotion() {
    return (
      typeof window !== 'undefined' &&
      typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-reduced-motion: reduce)').matches
    )
  }

  function animateTo(target) {
    if (rafId !== null) {
      cancelAnimationFrame(rafId)
      rafId = null
    }

    const to = Number(target) || 0
    const from = Number(display.value) || 0

    if (prefersReducedMotion() || duration <= 0 || from === to) {
      display.value = to
      return
    }

    const start = performance.now()
    const delta = to - from

    function tick(now) {
      const elapsed = Math.min(1, (now - start) / duration)
      const progress = easing(elapsed)
      display.value = Number((from + delta * progress).toFixed(decimals))
      if (elapsed < 1) {
        rafId = requestAnimationFrame(tick)
      } else {
        display.value = to
        rafId = null
      }
    }

    rafId = requestAnimationFrame(tick)
  }

  const watchSource = typeof source === 'function' ? source : source
  const stop = watch(watchSource, (val) => animateTo(val), { immediate: true })

  onBeforeUnmount(() => {
    if (rafId !== null) cancelAnimationFrame(rafId)
    stop()
  })

  return display
}

/** 三次缓出：快启动、慢收尾，契合数字滚动 */
export function easeOutCubic(t) {
  return 1 - Math.pow(1 - t, 3)
}

/** 二次缓出：更轻盈 */
export function easeOutQuad(t) {
  return 1 - (1 - t) * (1 - t)
}
