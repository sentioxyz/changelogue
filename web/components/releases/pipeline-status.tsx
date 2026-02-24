"use client";

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Badge } from "@/components/ui/badge";
import type { PipelineStatus } from "@/lib/api/types";
import { CheckCircle, Loader2, AlertCircle, Circle, XCircle } from "lucide-react";

const nodeLabels: Record<string, string> = {
  regex_normalizer: "Regex Normalizer",
  subscription_router: "Subscription Router",
  availability_checker: "Availability Checker",
  risk_assessor: "Risk Assessor",
  adoption_tracker: "Adoption Tracker",
  changelog_summarizer: "Changelog Summarizer",
  urgency_scorer: "Urgency Scorer",
  validation_trigger: "Validation Trigger",
};

const stateIcons: Record<string, React.ReactNode> = {
  completed: <CheckCircle className="h-5 w-5 text-green-600" />,
  running: <Loader2 className="h-5 w-5 animate-spin text-blue-600" />,
  discarded: <XCircle className="h-5 w-5 text-red-600" />,
  retry: <AlertCircle className="h-5 w-5 text-yellow-600" />,
  available: <Circle className="h-5 w-5 text-gray-400" />,
};

interface PipelineVisualizationProps {
  pipeline: PipelineStatus;
}

export function PipelineVisualization({ pipeline }: PipelineVisualizationProps) {
  const completedNodes = Object.keys(pipeline.node_results);

  return (
    <Card>
      <CardHeader className="flex flex-row items-center justify-between">
        <CardTitle className="text-base">Pipeline Status</CardTitle>
        <div className="flex items-center gap-2">
          {stateIcons[pipeline.state]}
          <Badge
            className={
              pipeline.state === "completed" ? "bg-green-100 text-green-800" :
              pipeline.state === "running" ? "bg-blue-100 text-blue-800" :
              pipeline.state === "discarded" ? "bg-red-100 text-red-800" :
              "bg-gray-100 text-gray-800"
            }
          >
            {pipeline.state}
          </Badge>
          {pipeline.current_node && (
            <span className="text-sm text-muted-foreground">
              Currently: {nodeLabels[pipeline.current_node] ?? pipeline.current_node}
            </span>
          )}
        </div>
      </CardHeader>
      <CardContent>
        <div className="space-y-3">
          {completedNodes.map((nodeName) => {
            const result = pipeline.node_results[nodeName] as Record<string, unknown>;
            return (
              <div key={nodeName} className="rounded-md border p-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <CheckCircle className="h-4 w-4 text-green-600" />
                    <span className="font-medium text-sm">
                      {nodeLabels[nodeName] ?? nodeName}
                    </span>
                  </div>
                </div>
                <div className="mt-2 grid gap-2">
                  {Object.entries(result).map(([key, value]) => (
                    <div key={key} className="flex gap-2 text-sm">
                      <span className="font-medium text-muted-foreground min-w-[140px]">
                        {key.replace(/_/g, " ")}:
                      </span>
                      <span>
                        {typeof value === "object"
                          ? JSON.stringify(value)
                          : String(value)}
                      </span>
                    </div>
                  ))}
                </div>
              </div>
            );
          })}

          {pipeline.current_node && !completedNodes.includes(pipeline.current_node) && (
            <div className="rounded-md border border-blue-200 bg-blue-50 p-4">
              <div className="flex items-center gap-2">
                <Loader2 className="h-4 w-4 animate-spin text-blue-600" />
                <span className="font-medium text-sm text-blue-800">
                  {nodeLabels[pipeline.current_node] ?? pipeline.current_node}
                </span>
                <span className="text-xs text-blue-600">Processing...</span>
              </div>
            </div>
          )}
        </div>

        {pipeline.attempt > 1 && (
          <div className="mt-3 text-xs text-muted-foreground">
            Attempt {pipeline.attempt} of 3
          </div>
        )}
      </CardContent>
    </Card>
  );
}
