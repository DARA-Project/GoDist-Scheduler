// Dara's modified version of the coverage tool.
// https://github.com/golang/tools/blob/master/cmd/cover/cover.go
// The initial source code copyright is as follows:
// Copyright 2013 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package instrumenter

import (
    "bytes"
    "fmt"
    "go/ast"
    "go/parser"
    "go/printer"
    "go/token"
    "io"
    "io/ioutil"
    "log"
    "os"
    "strconv"
    "strings"
)

// Block represents the information about a basic block to be recorded in the analysis.
// Note: Our definition of basic block is based on control structures; we don't break
// apart && and ||. We could but it doesn't seem important enough to bother.
type Block struct {
    startByte token.Pos
    endByte   token.Pos
    numStmt   int
}

// File is a wrapper for the state of a file used in the parser.
// The basic parse tree walker is a method of this type.
type File struct {
    fset      *token.FileSet
    name      string // Name of file.
    astFile   *ast.File
    blocks    []Block
    initialComments []byte
}

// Visit implements the ast.Visitor interface.
func (f *File) Visit(node ast.Node) ast.Visitor {
    switch n := node.(type) {
    case *ast.BlockStmt:
        // If it's a switch or select, the body is a list of case clauses; don't tag the block itself.
        if len(n.List) > 0 {
            switch n.List[0].(type) {
            case *ast.CaseClause: // switch
                for _, n := range n.List {
                    clause := n.(*ast.CaseClause)
                    clause.Body = f.addCounters(clause.Pos(), clause.End(), clause.Body, false)
                }
                return f
            case *ast.CommClause: // select
                for _, n := range n.List {
                    clause := n.(*ast.CommClause)
                    clause.Body = f.addCounters(clause.Pos(), clause.End(), clause.Body, false)
                }
                return f
            }
        }
        n.List = f.addCounters(n.Lbrace, n.Rbrace+1, n.List, true) // +1 to step past closing brace.
    case *ast.IfStmt:
        ast.Walk(f, n.Body)
        if n.Else == nil {
            return nil
        }
        // The elses are special, because if we have
        //  if x {
        //  } else if y {
        //  }
        // we want to cover the "if y". To do this, we need a place to drop the counter,
        // so we add a hidden block:
        //  if x {
        //  } else {
        //      if y {
        //      }
        //  }
        switch stmt := n.Else.(type) {
        case *ast.IfStmt:
            block := &ast.BlockStmt{
                Lbrace: n.Body.End(), // Start at end of the "if" block so the covered part looks like it starts at the "else".
                List:   []ast.Stmt{stmt},
                Rbrace: stmt.End(),
            }
            n.Else = block
        case *ast.BlockStmt:
            stmt.Lbrace = n.Body.End() // Start at end of the "if" block so the covered part looks like it starts at the "else".
        default:
            panic("unexpected node type in if")
        }
        ast.Walk(f, n.Else)
        return nil
    case *ast.SelectStmt:
        // Don't annotate an empty select - creates a syntax error.
        if n.Body == nil || len(n.Body.List) == 0 {
            return nil
        }
    case *ast.SwitchStmt:
        // Don't annotate an empty switch - creates a syntax error.
        if n.Body == nil || len(n.Body.List) == 0 {
            return nil
        }
    case *ast.TypeSwitchStmt:
        // Don't annotate an empty type switch - creates a syntax error.
        if n.Body == nil || len(n.Body.List) == 0 {
            return nil
        }
    }
    return f
}

// unquote returns the unquoted string.
func unquote(s string) string {
    t, err := strconv.Unquote(s)
    if err != nil {
        log.Fatalf("cover: improperly quoted string %q\n", s)
    }
    return t
}

// addImport adds an import for the specified path, if one does not already exist, and returns
// the local package name.
// For now, this is fixed to the runtime package as that is where the Dara's report coverage function is
func (f *File) addImport(path string) {
    // Does the package already import it?
    for _, s := range f.astFile.Imports {
        if unquote(s.Path.Value) == path {
            // The package is already imported woo
            return
        }
    }
    newImport := &ast.ImportSpec{
        Path: &ast.BasicLit{Value: strconv.Quote("runtime")},
    }
    impDecl := &ast.GenDecl{
        Tok: token.IMPORT,
        Specs: []ast.Spec{
            newImport,
        },
    }
    // Make the new import the first Decl in the file.
    astFile := f.astFile
    astFile.Decls = append(astFile.Decls, nil)
    copy(astFile.Decls[1:], astFile.Decls[0:])
    astFile.Decls[0] = impDecl
    astFile.Imports = append(astFile.Imports, newImport)
}

var slashslash = []byte("//")

// initialComments returns the prefix of content containing only
// whitespace and line comments.  Any +build directives must appear
// within this region.  This approach is more reliable than using
// go/printer to print a modified AST containing comments.
//
func initialComments(content []byte) []byte {
    // Derived from go/build.Context.shouldBuild.
    end := 0
    p := content
    for len(p) > 0 {
        line := p
        if i := bytes.IndexByte(line, '\n'); i >= 0 {
            line, p = line[:i], p[i+1:]
        } else {
            p = p[len(p):]
        }
        line = bytes.TrimSpace(line)
        if len(line) == 0 { // Blank line.
            end = len(content) - len(p)
            continue
        }
        if !bytes.HasPrefix(line, slashslash) { // Not comment line.
            break
        }
    }
    return content[:end]
}

// trimComments drops all but the //go: comments, some of which are semantically important.
// We drop all others because they can appear in places that cause our counters
// to appear in syntactically incorrect places. //go: appears at the beginning of
// the line and is syntactically safe.
func trimComments(file *ast.File, fset *token.FileSet) []*ast.CommentGroup {
    var comments []*ast.CommentGroup
    for _, group := range file.Comments {
        var list []*ast.Comment
        for _, comment := range group.List {
            if strings.HasPrefix(comment.Text, "//go:") && fset.Position(comment.Slash).Column == 1 {
                list = append(list, comment)
            }
        }
        if list != nil {
            comments = append(comments, &ast.CommentGroup{List: list})
        }
    }
    return comments
}

func (f *File) print(w io.Writer) {
    printer.Fprint(w, f.fset, f.astFile)
}

// intLiteral returns an ast.BasicLit representing the integer value.
func (f *File) intLiteral(i int) *ast.BasicLit {
    node := &ast.BasicLit{
        Kind:  token.INT,
        Value: fmt.Sprint(i),
    }
    return node
}

func (f *File) stringLiteral(val string) *ast.BasicLit {
    node := &ast.BasicLit{
        Kind: token.STRING,
        Value: fmt.Sprintf("%q", val),
    }
    return node
}

// index returns an ast.BasicLit representing the number of counters present.
func (f *File) index() *ast.BasicLit {
    return f.intLiteral(len(f.blocks))
}

func daraCounterStmt(f *File, counter ast.Expr) ast.Stmt {
    return &ast.ExprStmt{
        X: counter,
    }
}

func getBlockID(fname string, start int, end int) string {
    return fname + ":" + strconv.Itoa(start) + ":" + strconv.Itoa(end)
}

// newCounter creates a new counter expression of the appropriate form.
func (f *File) newCounter(start, end token.Pos, numStmt int) ast.Stmt {
    startLine := f.fset.Position(start)
    endLine := f.fset.Position(end)
    counter := &ast.CallExpr{
        Fun: &ast.SelectorExpr{
                X:   &ast.Ident{Name: "runtime"},
                Sel: &ast.Ident{Name: "ReportBlockCoverage"},
        },
        //Generate Block ID here instead of doing it at runtime
        Args: []ast.Expr{
            f.stringLiteral(getBlockID(f.name, startLine.Line, endLine.Line)),
        },
    }
    stmt := daraCounterStmt(f, counter)
    f.blocks = append(f.blocks, Block{start, end, numStmt})
    return stmt
}

// addCounters takes a list of statements and adds counters to the beginning of
// each basic block at the top level of that list. For instance, given
//
//  S1
//  if cond {
//      S2
//  }
//  S3
//
// counters will be added before S1 and before S3. The block containing S2
// will be visited in a separate call.
// TODO: Nested simple blocks get unnecessary (but correct) counters
func (f *File) addCounters(pos, blockEnd token.Pos, list []ast.Stmt, extendToClosingBrace bool) []ast.Stmt {
    // Special case: make sure we add a counter to an empty block. Can't do this below
    // or we will add a counter to an empty statement list after, say, a return statement.
    if len(list) == 0 {
        return []ast.Stmt{f.newCounter(pos, blockEnd, 0)}
    }
    // We have a block (statement list), but it may have several basic blocks due to the
    // appearance of statements that affect the flow of control.
    var newList []ast.Stmt
    for {
        // Find first statement that affects flow of control (break, continue, if, etc.).
        // It will be the last statement of this basic block.
        var last int
        end := blockEnd
        for last = 0; last < len(list); last++ {
            end = f.statementBoundary(list[last])
            if f.endsBasicSourceBlock(list[last]) {
                extendToClosingBrace = false // Block is broken up now.
                last++
                break
            }
        }
        if extendToClosingBrace {
            end = blockEnd
        }
        if pos != end { // Can have no source to cover if e.g. blocks abut.
            newList = append(newList, f.newCounter(pos, end, last))
        }
        newList = append(newList, list[0:last]...)
        list = list[last:]
        if len(list) == 0 {
            break
        }
        pos = list[0].Pos()
    }
    return newList
}

// hasFuncLiteral reports the existence and position of the first func literal
// in the node, if any. If a func literal appears, it usually marks the termination
// of a basic block because the function body is itself a block.
// Therefore we draw a line at the start of the body of the first function literal we find.
// TODO: what if there's more than one? Probably doesn't matter much.
func hasFuncLiteral(n ast.Node) (bool, token.Pos) {
    if n == nil {
        return false, 0
    }
    var literal funcLitFinder
    ast.Walk(&literal, n)
    return literal.found(), token.Pos(literal)
}

// statementBoundary finds the location in s that terminates the current basic
// block in the source.
func (f *File) statementBoundary(s ast.Stmt) token.Pos {
    // Control flow statements are easy.
    switch s := s.(type) {
    case *ast.BlockStmt:
        // Treat blocks like basic blocks to avoid overlapping counters.
        return s.Lbrace
    case *ast.IfStmt:
        found, pos := hasFuncLiteral(s.Init)
        if found {
            return pos
        }
        found, pos = hasFuncLiteral(s.Cond)
        if found {
            return pos
        }
        return s.Body.Lbrace
    case *ast.ForStmt:
        found, pos := hasFuncLiteral(s.Init)
        if found {
            return pos
        }
        found, pos = hasFuncLiteral(s.Cond)
        if found {
            return pos
        }
        found, pos = hasFuncLiteral(s.Post)
        if found {
            return pos
        }
        return s.Body.Lbrace
    case *ast.LabeledStmt:
        return f.statementBoundary(s.Stmt)
    case *ast.RangeStmt:
        found, pos := hasFuncLiteral(s.X)
        if found {
            return pos
        }
        return s.Body.Lbrace
    case *ast.SwitchStmt:
        found, pos := hasFuncLiteral(s.Init)
        if found {
            return pos
        }
        found, pos = hasFuncLiteral(s.Tag)
        if found {
            return pos
        }
        return s.Body.Lbrace
    case *ast.SelectStmt:
        return s.Body.Lbrace
    case *ast.TypeSwitchStmt:
        found, pos := hasFuncLiteral(s.Init)
        if found {
            return pos
        }
        return s.Body.Lbrace
    }
    // If not a control flow statement, it is a declaration, expression, call, etc. and it may have a function literal.
    // If it does, that's tricky because we want to exclude the body of the function from this block.
    // Draw a line at the start of the body of the first function literal we find.
    // TODO: what if there's more than one? Probably doesn't matter much.
    found, pos := hasFuncLiteral(s)
    if found {
        return pos
    }
    return s.End()
}

// endsBasicSourceBlock reports whether s changes the flow of control: break, if, etc.,
// or if it's just problematic, for instance contains a function literal, which will complicate
// accounting due to the block-within-an expression.
func (f *File) endsBasicSourceBlock(s ast.Stmt) bool {
    switch s := s.(type) {
    case *ast.BlockStmt:
        // Treat blocks like basic blocks to avoid overlapping counters.
        return true
    case *ast.BranchStmt:
        return true
    case *ast.ForStmt:
        return true
    case *ast.IfStmt:
        return true
    case *ast.LabeledStmt:
        return f.endsBasicSourceBlock(s.Stmt)
    case *ast.RangeStmt:
        return true
    case *ast.SwitchStmt:
        return true
    case *ast.SelectStmt:
        return true
    case *ast.TypeSwitchStmt:
        return true
    case *ast.ExprStmt:
        // Calls to panic change the flow.
        // We really should verify that "panic" is the predefined function,
        // but without type checking we can't and the likelihood of it being
        // an actual problem is vanishingly small.
        if call, ok := s.X.(*ast.CallExpr); ok {
            if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "panic" && len(call.Args) == 1 {
                return true
            }
        }
    }
    found, _ := hasFuncLiteral(s)
    return found
}

// funcLitFinder implements the ast.Visitor pattern to find the location of any
// function literal in a subtree.
type funcLitFinder token.Pos

func (f *funcLitFinder) Visit(node ast.Node) (w ast.Visitor) {
    if f.found() {
        return nil // Prune search.
    }
    switch n := node.(type) {
    case *ast.FuncLit:
        *f = funcLitFinder(n.Body.Lbrace)
        return nil // Prune search.
    }
    return f
}

func (f *funcLitFinder) found() bool {
    return token.Pos(*f) != token.NoPos
}

func (f *File) WriteAnnotatedFile(outfile string) error {
    fd, err := os.Create(outfile)
    if err != nil {
        return err
    }
    fd.Write(f.initialComments)
    f.print(fd)
    return nil
}

func Annotate(name string) (*File, error) {
    fset := token.NewFileSet()
    content, err := ioutil.ReadFile(name)
    if err != nil {
        return nil, err
    }
    parsedFile, err := parser.ParseFile(fset, name, content, parser.ParseComments)
    if err != nil {
        return nil, err
    }
    parsedFile.Comments = trimComments(parsedFile, fset)

    file := &File{
        fset:    fset,
        name:    name,
        astFile: parsedFile,
    }
    ast.Walk(file, file.astFile)
    file.addImport("runtime")
    file.initialComments = initialComments(content)
    return file, nil
}

