"use client";

import * as React from "react";

import { Card, CardContent } from "@/components/ui/card";
import { ExplorationGraph } from "@/components/exploration-graph";
import { api } from "@/lib/api";
import { startVisibleInterval } from "@/lib/visible-interval";
import type { Edge, TaskNode } from "@/lib/types";

export function GraphTab({ taskId }: { taskId: string }) {
  const [nodes, setNodes] = React.useState<TaskNode[]>([]);
  const [edges, setEdges] = React.useState<Edge[]>([]);
  const revRef = React.useRef("");

  React.useEffect(() => {
    let cancelled = false;
    revRef.current = "";
    const load = async () => {
      try {
        const rev = await api.explorationGraphRevision(taskId);
        if (cancelled) return;
        const sig = `${rev.node_count}:${rev.edge_count}:${rev.max_node_id}:${rev.max_edge_key}`;
        if (sig === revRef.current) return;
        const g = await api.explorationGraph(taskId);
        if (cancelled) return;
        revRef.current = sig;
        setNodes(g.nodes ?? []);
        setEdges(g.edges ?? []);
      } catch {
        /* keep last good data */
      }
    };
    const stop = startVisibleInterval(() => {
      void load();
    }, 20_000);
    return () => {
      cancelled = true;
      stop();
    };
  }, [taskId]);

  return (
    <Card>
      <CardContent className="p-0">
        <ExplorationGraph nodes={nodes} edges={edges} className="h-[72vh]" />
      </CardContent>
    </Card>
  );
}
