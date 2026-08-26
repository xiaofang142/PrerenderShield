import React, { useEffect, useRef } from 'react'
import echarts from './echarts'
import type { EChartsOption } from 'echarts'

interface BaseChartProps {
  option: EChartsOption
  height?: number
  style?: React.CSSProperties
  onChartReady?: (chart: echarts.ECharts) => void
}

const BaseChart: React.FC<BaseChartProps> = ({ option, height, style, onChartReady }) => {
  const chartRef = useRef<HTMLDivElement>(null)
  const chartInstanceRef = useRef<echarts.ECharts | null>(null)
  // 记录上次应用的 option 序列化结果，避免轮询场景下相同数据触发重绘
  const lastOptionJsonRef = useRef<string>('')
  const onChartReadyRef = useRef(onChartReady)
  onChartReadyRef.current = onChartReady

  // 挂载时初始化实例，卸载时销毁（实例常驻，不随父组件每次渲染重建）
  useEffect(() => {
    if (!chartRef.current) return

    const chart = echarts.init(chartRef.current)
    chartInstanceRef.current = chart

    const handleResize = () => { chart.resize() }
    window.addEventListener('resize', handleResize)

    const observer = new ResizeObserver(() => chart.resize())
    observer.observe(chartRef.current)

    return () => {
      window.removeEventListener('resize', handleResize)
      observer.disconnect()
      chart.dispose()
      chartInstanceRef.current = null
      lastOptionJsonRef.current = ''
    }
  }, [])

  // option 变化时增量应用（notMerge 保证无残留系列）
  useEffect(() => {
    const chart = chartInstanceRef.current
    if (!chart) return

    let json = ''
    try {
      json = JSON.stringify(option)
    } catch {
      json = ''
    }
    if (json !== '' && json === lastOptionJsonRef.current) return
    lastOptionJsonRef.current = json

    try {
      chart.setOption(option, true)
      onChartReadyRef.current?.(chart)
    } catch (error) {
      console.error('Failed to update chart:', error)
    }
  }, [option])

  return <div ref={chartRef} style={{ width: '100%', height: height ? `${height}px` : '100%', ...style }} />;
}

export default BaseChart
