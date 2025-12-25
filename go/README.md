# Everything SDK 3 - Go Binding

Go 语言绑定，用于与 [Everything 1.5](https://www.voidtools.com/) 搜索引擎进行 IPC 通信。

## 系统要求

- Windows 操作系统
- Everything 1.5 正在运行，且启用了 IPC
- Go 1.21 或更高版本

## 安装

```bash
go get github.com/voidtools/everything-sdk3-go/everything3
```

## 快速开始

### 简单搜索

```go
package main

import (
    "fmt"
    "github.com/voidtools/everything-sdk3-go/everything3"
)

func main() {
    // 连接到 Everything
    client, err := everything3.ConnectDefault()
    if err != nil {
        panic(err)
    }
    defer client.Close()

    // 创建搜索选项
    opts := everything3.DefaultSearchOptions()
    opts.Text = "*.go"
    opts.Count = 10

    // 执行搜索
    results, err := client.Search(opts)
    if err != nil {
        panic(err)
    }

    // 打印结果
    fmt.Printf("找到 %d 个结果\n", results.TotalCount)
    for _, result := range results.Results {
        fmt.Printf("%s\n", result.FullPath)
    }
}
```

### 分页查询

```go
opts := everything3.DefaultSearchOptions()
opts.Text = "document"
opts.Offset = 20  // 跳过前 20 个结果
opts.Count = 10   // 每页 10 个结果

results, _ := client.Search(opts)
```

### 排序结果

```go
opts := everything3.DefaultSearchOptions()
opts.Text = "*.mp3"
opts.Sorts = []everything3.Sort{
    {
        PropertyID: everything3.PropertyIDSize,
        Ascending:  false, // 最大文件优先
    },
}
```

### 高级搜索选项

```go
opts := &everything3.SearchOptions{
    Text:            "config",
    MatchCase:       true,   // 区分大小写
    MatchWholeWords: true,   // 全字匹配
    MatchPath:       true,   // 搜索路径
    UseRegex:        false,  // 使用正则表达式
    FoldersFirst:    everything3.FoldersFirstYes,
    Count:           50,
}
```

### 获取文件信息

```go
// 获取文件属性
attrs, err := client.GetFileAttributes(`C:\Windows\notepad.exe`)
if err == nil {
    fmt.Printf("属性: 0x%08X\n", attrs)
}

// 获取详细文件信息
info, err := client.GetFileInfo(`C:\Windows\notepad.exe`)
if err == nil {
    fmt.Printf("大小: %s\n", everything3.FormatSize(info.Size))
    fmt.Printf("修改时间: %s\n", info.LastWriteTime)
}
```

### 遍历文件

```go
// 使用 FindFirstFile/FindNextFile 模式
handle, firstFile, err := client.FindFirstFile(`C:\Windows\*.exe`)
if err != nil {
    panic(err)
}
defer handle.Close()

fmt.Println(firstFile.Name)

for {
    file, err := handle.Next()
    if err != nil {
        break // 没有更多文件
    }
    fmt.Println(file.Name)
}
```

### 获取文件哈希值

```go
// 获取单个哈希值
crc32, err := client.GetFileCRC32(`C:\Windows\notepad.exe`)
if err == nil {
    fmt.Printf("CRC32: %08X\n", crc32)
}

md5, err := client.GetFileMD5(`C:\Windows\notepad.exe`)
if err == nil {
    fmt.Printf("MD5: %s\n", md5)
}

sha256, err := client.GetFileSHA256(`C:\Windows\notepad.exe`)
if err == nil {
    fmt.Printf("SHA256: %s\n", sha256)
}

// 获取所有可用的哈希值
hashes, err := client.GetFileHashes(`C:\Windows\notepad.exe`)
if err == nil {
    if hashes.CRC32.Valid {
        fmt.Printf("CRC32: %08X\n", hashes.CRC32.CRC32())
    }
    if hashes.MD5.Valid {
        fmt.Printf("MD5: %s\n", hashes.MD5.String())
    }
    if hashes.SHA256.Valid {
        fmt.Printf("SHA256: %s\n", hashes.SHA256.String())
    }
}
```

### 搜索时获取哈希值

```go
// 使用包含哈希属性的搜索选项
opts := everything3.SearchOptionsWithHashes()
opts.Text = "*.exe"
opts.Count = 10

results, err := client.Search(opts)
if err == nil {
    for _, result := range results.Results {
        fmt.Printf("File: %s\n", result.Name)
        if result.CRC32.Valid {
            fmt.Printf("  CRC32: %08X\n", result.CRC32.CRC32())
        }
        if result.SHA256.Valid {
            fmt.Printf("  SHA256: %s\n", result.SHA256.String())
        }
    }
}
```

## API 参考

### 连接管理

| 函数 | 描述 |
|------|------|
| `Connect(instanceName string)` | 连接到指定实例 |
| `ConnectDefault()` | 连接到默认实例（自动尝试 "1.5a"） |
| `client.Close()` | 关闭连接 |

### 搜索

| 函数 | 描述 |
|------|------|
| `client.Search(opts)` | 执行搜索 |
| `DefaultSearchOptions()` | 获取默认搜索选项 |

### 文件操作

| 函数 | 描述 |
|------|------|
| `client.GetFileAttributes(path)` | 获取文件属性 |
| `client.GetFileInfo(path)` | 获取详细文件信息 |
| `client.FindFirstFile(pattern)` | 开始文件遍历 |
| `handle.Next()` | 获取下一个文件 |
| `handle.Close()` | 关闭遍历句柄 |

### 哈希/校验和

| 函数 | 描述 |
|------|------|
| `client.GetFileCRC32(path)` | 获取 CRC32 校验值 |
| `client.GetFileCRC64(path)` | 获取 CRC64 校验值 |
| `client.GetFileMD5(path)` | 获取 MD5 哈希（十六进制字符串） |
| `client.GetFileSHA1(path)` | 获取 SHA1 哈希 |
| `client.GetFileSHA256(path)` | 获取 SHA256 哈希 |
| `client.GetFileSHA512(path)` | 获取 SHA512 哈希 |
| `client.GetFileHashes(path)` | 获取所有可用的哈希值 |
| `client.GetFileHash(path, propertyID)` | 获取指定类型的哈希值 |

### 通用属性查询

| 函数 | 描述 |
|------|------|
| `client.GetPropertyString(path, id)` | 获取字符串属性 |
| `client.GetPropertyUint32(path, id)` | 获取 32 位整数属性 |
| `client.GetPropertyUint64(path, id)` | 获取 64 位整数属性 |

### 属性 ID

基本属性：

| 常量 | 值 | 描述 |
|------|-----|------|
| `PropertyIDName` | 0 | 文件名 |
| `PropertyIDPath` | 1 | 文件夹路径 |
| `PropertyIDSize` | 2 | 文件大小 |
| `PropertyIDExtension` | 3 | 文件扩展名 |
| `PropertyIDDateModified` | 5 | 修改时间 |
| `PropertyIDDateCreated` | 6 | 创建时间 |
| `PropertyIDDateAccessed` | 7 | 访问时间 |
| `PropertyIDAttributes` | 8 | 文件属性 |

哈希/校验和属性：

| 常量 | 值 | 描述 |
|------|-----|------|
| `PropertyIDCRC32` | 40 | CRC32 校验值 (4 字节) |
| `PropertyIDCRC64` | 244 | CRC64 校验值 (8 字节) |
| `PropertyIDMD5` | 37 | MD5 哈希 (16 字节) |
| `PropertyIDSHA1` | 38 | SHA1 哈希 (20 字节) |
| `PropertyIDSHA256` | 39 | SHA256 哈希 (32 字节) |
| `PropertyIDSHA384` | 243 | SHA384 哈希 (48 字节) |
| `PropertyIDSHA512` | 242 | SHA512 哈希 (64 字节) |

文件夹哈希属性（计算文件夹内容的哈希）：

| 常量 | 值 | 描述 |
|------|-----|------|
| `PropertyIDFolderDataCRC32` | 361 | 文件夹数据 CRC32 |
| `PropertyIDFolderDataMD5` | 363 | 文件夹数据 MD5 |
| `PropertyIDFolderDataSHA256` | 365 | 文件夹数据 SHA256 |

## 运行示例

```bash
cd go/example
go run main.go "*.txt"
```

## 许可证

MIT License - 与原始 Everything SDK 3 相同
