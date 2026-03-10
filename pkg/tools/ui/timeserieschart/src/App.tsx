import { useState } from 'react'
import { useApp } from '@modelcontextprotocol/ext-apps/react'
import type { CallToolResult } from '@modelcontextprotocol/sdk/types.js'
import { z } from 'zod'
import './App.css'

const timeSeriesChartArgsSchema = z.object({
  project_id: z.string().optional(),
  filter: z.string(),
  start_time: z.string().optional(),
  end_time: z.string().optional(),
  title: z.string().optional(),
})

function App() {
  const [mcpData, setMcpData] = useState<any>(null)
  const [loading, setLoading] = useState<boolean>(true)
  const [errorMsg, setErrorMsg] = useState<string>('')
  const [title, setTitle] = useState<string>('Timeseries Data Viewer')

  // MCP UI App Configuration
  useApp({
    appInfo: {
      name: 'Time Series Chart',
      version: '1.0.0',
    },
    capabilities: {},
    onAppCreated: (appInstance) => {
      // Receive input when launched by the MCP host
      appInstance.ontoolinput = async (request) => {
        try {
          // Verify arguments received using Zod schema
          const parseResult = timeSeriesChartArgsSchema.safeParse(request.arguments)
          if (!parseResult.success) {
            setErrorMsg(`Invalid time series parameters provided in tool input:\n${parseResult.error.message}`)
            setLoading(false)
            return
          }


          const args = parseResult.data

          if (args.title) {
            setTitle(args.title)
          }

          // Execute internal data-fetch tool 
          const response = await appInstance.callServerTool({
            name: 'list_time_series',
            arguments: args
          }) as CallToolResult

          console.log('response: ', response)

          if (response.isError) {
            const errorText = response.content?.[0]?.type === 'text' ? response.content[0].text : 'Unknown Error';
            setErrorMsg(`Error fetching time series: ${errorText}`)
          } else {
            // Extract JSON from the raw TextContent output
            const contentText = response.content?.[0]?.type === 'text' ? response.content[0].text : '{}';
            setMcpData(JSON.parse(contentText))
          }
        } catch (err: any) {
          setErrorMsg(`Failed to call time series API: ${err.message}`)
        } finally {
          setLoading(false)
        }
      }
    }
  })

  if (loading) {
    return <div className="App"><h2>Loading Time Series Data...</h2></div>
  }

  if (errorMsg) {
    return <div className="App" style={{ color: 'red' }}><h2>Error</h2><p>{errorMsg}</p></div>
  }

  return (
    <div className="App">
      <h2>{title}</h2>
      <div className="card">
        <pre style={{ textAlign: 'left', whiteSpace: 'pre-wrap', maxHeight: '500px', overflowY: 'auto' }}>
          {JSON.stringify(mcpData, null, 2)}
        </pre>
      </div>
    </div>
  )
}

export default App
