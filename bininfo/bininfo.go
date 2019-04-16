package bininfo

import (
    "debug/dwarf"
    "fmt"
    "github.com/go-delve/delve/pkg/proc"
    "strings"
)

var DefaultPackages = map[string]bool{
    "archive"                   : true,
    "archive/tar"               : true,
    "archive/zip"               : true,
    "bufio"                     : true,
    "builtin"                   : true,
    "bytes"                     : true,
    "compress"                  : true,
    "compress/bzip2"            : true,
    "compress/flate"            : true,
    "compress/gzip"             : true,
    "compress/lzw"              : true,
    "compress/zlib"             : true,
    "container/heap"            : true,
    "container/list"            : true,
    "container/ring"            : true,
    "context"                   : true,
    "crypto"                    : true,
    "crypto/aes"                : true,
    "crypto/cipher"             : true,
    "crypto/des"                : true,
    "crypto/dsa"                : true,
    "crypto/ecdsa"              : true,
    "crypto/elliptic"           : true,
    "crypto/hmac"               : true,
    "crypto/internal/cipherhw"  : true,
    "crypto/md5"                : true,
    "crypto/rand"               : true,
    "crypto/rc4"                : true,
    "crypto/rsa"                : true,
    "crypto/sha1"               : true,
    "crypto/sha256"             : true,
    "crypto/sha512"             : true,
    "crypto/subtle"             : true,
    "crypto/tls"                : true,
    "crypto/x509"               : true,
    "crypto/x509/pkix"          : true,
    "database"                  : true,
    "database/sql"              : true,
    "database/sql/driver"       : true,
    "debug"                     : true,
    "debug/dwarf"               : true,
    "debug/elf"                 : true,
    "debug/gosym"               : true,
    "debug/macho"               : true,
    "debug/pe"                  : true,
    "debug/plan9obj"            : true,
    "encoding"                  : true,
    "encoding/ascii85"          : true,
    "encoding/asn1"             : true,
    "encoding/base32"           : true,
    "encoding/base64"           : true,
    "encoding/binary"           : true,
    "encoding/csv"              : true,
    "encoding/gob"              : true,
    "encoding/hex"              : true,
    "encoding/json"             : true,
    "encoding/pem"              : true,
    "encoding/xml"              : true,
    "expvar"                    : true,
    "flag"                      : true,
    "fmt"                       : true,
    "go"                        : true,
    "go/ast"                    : true,
    "go/build"                  : true,
    "go/constant"               : true,
    "go/doc"                    : true,
    "go/format"                 : true,
    "go/importer"               : true,
    "go/parser"                 : true,
    "go/printer"                : true,
    "go/scanner"                : true,
    "go/token"                  : true,
    "go/types"                  : true,
    "hash"                      : true,
    "hash/adler32"              : true,
    "hash/crc32"                : true,
    "hash/crc64"                : true,
    "hash/fnv"                  : true,
    "html"                      : true,
    "html/template"             : true,
    "image"                     : true,
    "image/color"               : true,
    "image/color/palette"       : true,
    "image/draw"                : true,
    "image/gif"                 : true,
    "image/jpeg"                : true,
    "image/png"                 : true,
    "index"                     : true,
    "index/suffixarray"         : true,
    "internal/cpu"              : true,
    "internal/poll"             : true,
    "internal/nettrace"         : true,
    "internal/race"             : true,
    "internal/singleflight"     : true,
    "internal/syscall"          : true,
    "internal/syscall/unix"     : true,
    "internal/syscall/windows"  : true,
    "internal/testlog"          : true,
    "internal/testenv"          : true,
    "internal/trace"            : true,
    "io"                        : true,
    "io/ioutil"                 : true,
    "log"                       : true,
    "log/syslog"                : true,
    "math"                      : true,
    "math/big"                  : true,
    "math/bits"                 : true,
    "math/cmplx"                : true,
    "math/rand"                 : true,
    "mime"                      : true,
    "mime/multipart"            : true,
    "mime/quotedprintable"      : true,
    "net"                       : true,
    "net/http"                  : true,
    "net/http/cgi"              : true,
    "net/http/cookiejar"        : true,
    "net/http/fcgi"             : true,
    "net/http/internal"         : true,
    "net/http/httptest"         : true,
    "net/http/httptrace"        : true,
    "net/http/httputil"         : true,
    "net/http/pprof"            : true,
    "net/mail"                  : true,
    "net/rpc"                   : true,
    "net/rpc/jsonrpc"           : true,
    "net/smtp"                  : true,
    "net/textproto"             : true,
    "net/url"                   : true,
    "os"                        : true,
    "os/exec"                   : true,
    "os/signal"                 : true,
    "os/user"                   : true,
    "path"                      : true,
    "path/filepath"             : true,
    "plugin"                    : true,
    "reflect"                   : true,
    "regexp"                    : true,
    "regexp/syntax"             : true,
    "runtime"                   : true,
    "runtime/cgo"               : true,
    "runtime/debug"             : true,
    "runtime/msan"              : true,
    "runtime/pprof"             : true,
    "runtime/race"              : true,
    "runtime/trace"             : true,
    "sort"                      : true,
    "strconv"                   : true,
    "strings"                   : true,
    "sync"                      : true,
    "sync/atomic"               : true,
    "syscall"                   : true,
    "syscall/js"                : true,
    "testing"                   : true,
    "testing/iotest"            : true,
    "testing/quick"             : true,
    "text"                      : true,
    "text/scanner"              : true,
    "text/tabwriter"            : true,
    "text/template"             : true,
    "text/template/parse"       : true,
    "time"                      : true,
    "unicode"                   : true,
    "unciode/utf16"             : true,
    "unicode/utf8"              : true,
    "unsafe"                    : true,
}

var OtherPackages = map[string]bool {
    "errors"                : true,
    "benchmarks"            : true,
    "exp"                   : true,
}

func IsUserCompileUnit(name string) bool {
    if strings.Contains(name, "vendor") {
        return false
    }

    if name == "dara" {
        return false
    }

    if _, ok := DefaultPackages[name]; ok {
        return false
    }

    if _, ok := OtherPackages[name]; ok {
        return false
    }

    return true
}

func PrintBinaryInfo(exePath string, debugDirectories []string) error {
    bi := proc.NewBinaryInfo("linux", "amd64")
    err := bi.LoadBinaryInfo(exePath, 0, debugDirectories)
    if err != nil {
        return err
    }
    if err == nil {
        err = bi.LoadError()
    }
    if err != nil {
        return err
    }
    types, err := bi.Types()
    if err != nil {
        return err
    }
    fmt.Println("Binary has",len(types) ,"types")
    //for _, t := range types {
    //    fmt.Println(t)
    //}
    //scope := proc.GlobalScope(bi)
    //fmt.Println(scope)
    //locals, err := scope.Locals()
    //fmt.Println("Got", len(locals), "locals")
    reader := bi.DwarfReader()
    for {
        entry, err := reader.Next()
        if err != nil {
            return err
        }
        if entry == nil {
            break
        }
        //entry2, name, typ, err := proc.ReadVarEntry(entry, bi)
        //if err != nil {
        //    return err
        //}
        //fmt.Println(entry2, name, typ)
        //if entry.Tag == dwarf.TagFormalParameter || entry.Tag == dwarf.TagVariable {
        if entry.Tag == dwarf.TagCompileUnit {
            name, ok := entry.Val(dwarf.AttrName).(string)
            if ok {
                if IsUserCompileUnit(name) {
                    fmt.Println("Compile Unit :",name)
                } else {
                    reader.SkipChildren()
                }
            }
        }
        if entry.Tag == dwarf.TagSubprogram {
            name, ok := entry.Val(dwarf.AttrName).(string)
            if ok {
                fmt.Println("\t", name)
            }
        }
        if entry.Tag == dwarf.TagFormalParameter {
            name, ok := entry.Val(dwarf.AttrName).(string)
            if ok {
                fmt.Println("\t\t Param :", name)
            }
        }
        if entry.Tag == dwarf.TagVariable {
            name, ok := entry.Val(dwarf.AttrName).(string)
            if ok {
                fmt.Println("\t\t Variable :", name)
            }
        }
    }
    return nil
}
