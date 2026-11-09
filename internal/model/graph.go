package model

// Graph es el árbol aplanado: la forma en que las plantillas de salida ven
// el modelo. Un árbol con N nodos da un grafo con N nodos y N-1 aristas
// (una por cada relación padre → hijo).
//
// Campos:
//   - Nodes: nodos en preorden, con IDs consecutivos desde 0.
//   - Edges: aristas padre → hijo, en el orden en que se recorren.
//
// La operación Flatten, que construye un Graph a partir de un *Node, se
// añadirá a este fichero junto con el primer escritor (fase 3).
type Graph struct {
	Nodes []GraphNode
	Edges []GraphEdge
}

// GraphNode es un nodo del grafo aplanado.
//
// Campos:
//   - ID: identificador del nodo en el grafo (posición en preorden).
//   - Label: texto que se mostrará en la salida.
//   - Kind: clase del nodo de origen en el árbol.
type GraphNode struct {
	ID    int
	Label string
	Kind  NodeKind
}

// GraphEdge es una arista dirigida del grafo aplanado: From es el ID del
// padre y To el ID del hijo.
type GraphEdge struct {
	From int
	To   int
}
