import { t } from "@lingui/core/macro"
import { Fragment, useState } from "react"
import LineChartDefault from "@/components/charts/line-chart"
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from "@/components/ui/card"
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select"
import { cn, decimalString } from "@/lib/utils"
import type { ChartData, SystemRecord, SystemStatsRecord, UPSStats } from "@/types"
import { ChartCard } from "../chart-card"

function statusLabel(flag: string) {
	switch (flag) {
		case "OL": return t`Utility power`
		case "OB": return t`On battery`
		case "LB": return t`Low battery`
		case "RB": return t`Replace battery`
		case "OVER": return t`Overload`
		case "CHRG": return t`Charging`
		case "DISCHRG": return t`Discharging`
		case "BYPASS": return t`Bypass`
		case "OFF": return t`Output off`
		case "FSD": return t`Forced shutdown`
		case "ALARM": return t`UPS alarm`
		default: return flag
	}
}

export function UPSCharts({ chartData, grid, dataEmpty, maxValues, system }: {
	chartData: ChartData
	grid: boolean
	dataEmpty: boolean
	maxValues: boolean
	system: SystemRecord
}) {
	const [selected, setSelected] = useState("")
	const ids = new Set(Object.keys(system.info.ups ?? {}))
	for (const record of chartData.systemStats) {
		for (const id of Object.keys(record.stats?.ups ?? {})) ids.add(id)
	}
	const names = [...ids].sort()
	const id = ids.has(selected) ? selected : names[0]
	if (!id) return null
	const current = system.info.ups?.[id]
	// Current readings always come from live system info, independently of the chart range.
	const fresh = system.status === "up" && current?.online && Date.now() / 1000 - current.updated < 120
	const flags = fresh ? current.status?.split(/\s+/).filter(Boolean) ?? [] : []
	const warning = !fresh || flags.some((flag) => ["OB", "LB", "RB", "OVER", "FSD", "ALARM", "OFF"].includes(flag))
	const metrics = fresh ? current?.metrics : undefined
	const groups = [
		{ title: t`UPS charge and load`, unit: "%", keys: ["battery.charge", "ups.load"], labels: [t`Battery charge`, t`UPS load`], scale: 1 },
		{ title: t`UPS runtime`, unit: t`min`, keys: ["battery.runtime"], labels: [t`Estimated runtime`], scale: 60 },
		{ title: t`UPS power`, unit: "W", keys: ["ups.realpower"], labels: [t`Power`], scale: 1 },
		{ title: t`UPS voltage`, unit: "V", keys: ["input.voltage", "output.voltage", "battery.voltage"], labels: [t`Input voltage`, t`Output voltage`, t`Battery voltage`], scale: 1 },
		{ title: t`UPS temperature`, unit: "°C", keys: ["ups.temperature", "battery.temperature"], labels: [t`UPS temperature`, t`Battery temperature`], scale: 1 },
	]
	const reading = (key: string, unit: string, scale = 1) => {
		const value = metrics?.[key]
		return value === undefined ? "—" : `${decimalString(value / scale, 1)} ${unit}`
	}
	return (
		<Fragment>
			<Card className="col-span-full">
				<CardHeader>
					<div className="flex flex-wrap items-center justify-between gap-3">
						<CardTitle>UPS · {id}</CardTitle>
						{names.length > 1 && <Select value={id} onValueChange={setSelected}>
							<SelectTrigger className="w-48" aria-label={t`Select UPS`}><SelectValue /></SelectTrigger>
							<SelectContent>{names.map((name) => <SelectItem key={name} value={name}>{name}</SelectItem>)}</SelectContent>
						</Select>}
					</div>
					<CardDescription>{current?.model || id}</CardDescription>
					<p className={cn("text-sm font-medium", warning ? "text-destructive" : "text-emerald-600 dark:text-emerald-400")}>
						{fresh ? flags.map(statusLabel).join(" · ") || t`Unknown` : t`UPS unavailable or data stale`}
					</p>
					<CardDescription>{t`Last successful reading`}: {current?.updated ? new Date(current.updated * 1000).toLocaleString() : "—"}</CardDescription>
				</CardHeader>
				<CardContent>
					<dl className="grid grid-cols-2 gap-4 sm:grid-cols-4 tabular-nums">
						{[[t`Battery charge`, reading("battery.charge", "%")], [t`Estimated runtime`, reading("battery.runtime", t`min`, 60)], [t`UPS load`, reading("ups.load", "%")], [t`Power`, reading("ups.realpower", "W")]].map(([label, value]) => (
							<div key={label}><dt className="text-sm text-muted-foreground">{label}</dt><dd className="mt-1 text-xl font-semibold">{value}</dd></div>
						))}
					</dl>
				</CardContent>
			</Card>
			{groups.map((group) => {
				const keys = group.keys.filter((key) => chartData.systemStats.some((record) => record.stats?.ups?.[id]?.metrics?.[key] !== undefined))
				if (!keys.length) return null
				return <ChartCard key={`${id}:${group.title}`} empty={dataEmpty} grid={grid} title={group.title} description={id}>
					<LineChartDefault
						key={`${id}:${maxValues}`}
						chartData={chartData}
						maxToggled={maxValues}
						legend={keys.length > 1}
						dataPoints={keys.map((key, index) => ({
							label: group.labels[group.keys.indexOf(key)],
							color: index + 1,
							dataKey: ({ stats }: SystemStatsRecord) => {
								const sample: UPSStats | undefined = stats?.ups?.[id]
								const value = (maxValues ? sample?.max?.[key] : undefined) ?? sample?.metrics?.[key]
								return value === undefined ? undefined : value / group.scale
							},
						}))}
						tickFormatter={(value) => `${decimalString(value, 1)} ${group.unit}`}
						contentFormatter={({ value }) => `${decimalString(value, 1)} ${group.unit}`}
					/>
				</ChartCard>
			})}
		</Fragment>
	)
}
