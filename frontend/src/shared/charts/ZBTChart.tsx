import * as echarts from 'echarts/core'
import { BarChart, LineChart, PieChart } from 'echarts/charts'
import { GridComponent, LegendComponent, TooltipComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'
import ReactEChartsCore from 'echarts-for-react/esm/core'
import type { EChartsReactProps } from 'echarts-for-react/esm/types'

echarts.use([BarChart, LineChart, PieChart, GridComponent, LegendComponent, TooltipComponent, CanvasRenderer])

export function ZBTChart(props: Omit<EChartsReactProps, 'echarts'>) {
  return <ReactEChartsCore echarts={echarts} {...props} />
}
