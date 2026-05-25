package daemon

import "encoding/json"

var ParseNamespace = parseNamespace

func (s *Server) FindFileForDecl(dir, declName string) (string, error) {
	return s.findFileForDecl(dir, declName)
}

func (s *Server) ResolveNamespace(tool string, raw json.RawMessage) (json.RawMessage, error) {
	return s.resolveNamespace(tool, raw)
}

func NewTestServer(dir string) *Server {
	return &Server{Dir: dir}
}
