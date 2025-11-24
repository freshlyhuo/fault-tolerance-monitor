/* 定义业务层 PDU 结构： */
package models

// ------------------------------------------------------------
// PDU 基础类型与事件接口
// ------------------------------------------------------------

// PDUType 定义顶层识别类型：数据/指令
// 文档规定：
// 数据文件/配置文件/安装包文件 = 0x01/0x02/0x03
// 指令类 = 0x04
type PDUType uint8

const (
    PDUTypeDataFile       PDUType = 0x01
    PDUTypeConfigFile     PDUType = 0x02
    PDUTypePackageFile    PDUType = 0x03
    PDUTypeCommand        PDUType = 0x04
)

// CommandSubtype 指令类型码（第二字节）
type CommandSubtype uint8

const (
    CmdMetaPDU                       CommandSubtype = 0x01
    CmdMetaFailPDU                   CommandSubtype = 0xA1
    CmdEOF                           CommandSubtype = 0x02
    CmdTransferCompleted             CommandSubtype = 0xA2
    CmdTransferFailed                CommandSubtype = 0xB2
    CmdResumePoint                   CommandSubtype = 0x03
    CmdNAK                           CommandSubtype = 0x04
    CmdStorageQuery                  CommandSubtype = 0x05
    CmdStorageQueryResp              CommandSubtype = 0xA5
    CmdDeleteFile                    CommandSubtype = 0x06
    CmdDeleteFileResp                CommandSubtype = 0xA6
    CmdQueryFileList                 CommandSubtype = 0x07
    CmdQueryFileListResp             CommandSubtype = 0xA7
    CmdDownloadFile                  CommandSubtype = 0x08
    CmdDeltaUpgrade                  CommandSubtype = 0x09
    CmdUpgradeSuccess                CommandSubtype = 0xA9
    CmdUpgradeFailed                 CommandSubtype = 0xB9
)

// PDUEvent — 所有 PDU 的通用接口
type PDUEvent interface {
    GetType() PDUType
    GetSessionID() uint32
}

// BasePDU — 所有 PDU 公共元数据
type BasePDU struct {
    Type      PDUType // 第一字节
    CRC       uint16  // 最后2字节校验位（解析器填入）
    RawLength int     // 原始帧长度，用于 Debug
}

func (b BasePDU) GetType() PDUType { return b.Type }

// ------------------------------------------------------------
// (1) File Data PDU — 数据文件 / 配置文件 / 软件包
// ------------------------------------------------------------
//
// 字节序：
// 1       类型标识（0x01/0x02/0x03）
// 2~5     会话ID
// 6~7     偏移量
// 8~9     数据长度
// 10~...  数据体
// ...     CRC
//------------------------------------------------------------

type FileDataPDU struct {
    BasePDU
    SessionID   uint32
    Offset      uint16
    DataLength  uint16
    Data        []byte
}

func (p *FileDataPDU) GetSessionID() uint32 { return p.SessionID }

// ------------------------------------------------------------
// (2) 元数据 PDU（指令类型）
// ------------------------------------------------------------
//
// 字节序：
// 1      = 0x04
// 2      = 0x01
// 3~6    SessionID
// 7      文件名长度
// 8~...  文件名
// ...    文件长度（4字节）
// ...    文件类型
// ...    软件ID（8字节）
// ...    软件版本号（4）
// ...    数字签名（8）
// ...    CRC
//------------------------------------------------------------

type MetaPDU struct {
    BasePDU
    SessionID      uint32
    FileName       string
    FileLength     uint32
    FileType       uint8
    SoftwareID     uint64
    SoftwareVer    uint32
    Signature      []byte // 8字节
}

func (p *MetaPDU) GetSessionID() uint32 { return p.SessionID }

// ------------------------------------------------------------
// (3) 元数据协商失败 PDU
// ------------------------------------------------------------

type MetaFailPDU struct {
    BasePDU
    SessionID uint32
    ErrorCode uint8 // 1:签名失败 2:哈希失败 3:空间不足 4:文件名冲突
}

func (p *MetaFailPDU) GetSessionID() uint32 { return p.SessionID }

// ------------------------------------------------------------
// (4) EOF PDU
// ------------------------------------------------------------
//
// 字节序：
// 2 = 0x02
// 3~6 sessionID
// 7 状态码
// 8~11 最终文件大小
// 12~13 文件校验和
//------------------------------------------------------------

type EOFPDU struct {
    BasePDU
    SessionID   uint32
    Status      uint8
    FinalSize   uint32
    FileChecksum uint16
}

func (p *EOFPDU) GetSessionID() uint32 { return p.SessionID }

// ------------------------------------------------------------
// (5) Transfer Completed
// ------------------------------------------------------------

type TransferCompletedPDU struct {
    BasePDU
    SessionID uint32
    FileSize  uint32
    Checksum  uint16
    Timestamp uint64 // 8字节
}

func (p *TransferCompletedPDU) GetSessionID() uint32 { return p.SessionID }

// ------------------------------------------------------------
// (6) Transfer Failed
// ------------------------------------------------------------

type TransferFailedPDU struct {
    BasePDU
    SessionID uint32
    ErrorCode uint8
    Timestamp uint64
}

func (p *TransferFailedPDU) GetSessionID() uint32 { return p.SessionID }

// ------------------------------------------------------------
// (7) 续传点 PDU
// ------------------------------------------------------------

type ResumePointPDU struct {
    BasePDU
    SessionID    uint32
    NextOffset   uint16
    CurrentSize  uint16
}

func (p *ResumePointPDU) GetSessionID() uint32 { return p.SessionID }

// ------------------------------------------------------------
// (8) NAK PDU
// ------------------------------------------------------------

type NAKPDU struct {
    BasePDU
    SessionID uint32
    Offset    uint8
}

func (p *NAKPDU) GetSessionID() uint32 { return p.SessionID }

// ------------------------------------------------------------
// (9) 存储空间查询 PDU
// ------------------------------------------------------------

type StorageQueryPDU struct {
    BasePDU
    CommandID  uint64
    Signature  []byte // 8字节
}

func (p *StorageQueryPDU) GetSessionID() uint32 { return 0 }

// ------------------------------------------------------------
// (10) 存储空间查询响应
// ------------------------------------------------------------

type StorageQueryRespPDU struct {
    BasePDU
    CommandID uint64
    Status    uint8
    PartitionCount uint8
    Partitions []PartitionInfo // 每分区结构
}

type PartitionInfo struct {
    PartitionID   uint8
    TotalSpace    uint32
    FreeSpace     uint32
}

func (p *StorageQueryRespPDU) GetSessionID() uint32 { return 0 }

// ------------------------------------------------------------
// (11) 删除文件指令
// ------------------------------------------------------------

type DeleteFilePDU struct {
    BasePDU
    CommandID    uint64
    FileName     string
    FileHash     uint16
    Signature    []byte // 8字节
}

func (p *DeleteFilePDU) GetSessionID() uint32 { return 0 }

// ------------------------------------------------------------
// (12) 删除文件响应
// ------------------------------------------------------------

type DeleteFileRespPDU struct {
    BasePDU
    CommandID     uint64
    Status        uint8
    ActualHash    uint16
    RequestHash   uint16
}

func (p *DeleteFileRespPDU) GetSessionID() uint32 { return 0 }

// ------------------------------------------------------------
// (13) 查询文件列表指令
// ------------------------------------------------------------

type QueryFileListPDU struct {
    BasePDU
    CommandID   uint64
    Directory   uint32
    TargetName  uint32
    Signature   []byte
}

func (p *QueryFileListPDU) GetSessionID() uint32 { return 0 }

// ------------------------------------------------------------
// (14) 文件列表响应
// ------------------------------------------------------------

type QueryFileListRespPDU struct {
    BasePDU
    CommandID  uint64
    Status     uint8
    Directory  uint32
    TargetName uint32
}

func (p *QueryFileListRespPDU) GetSessionID() uint32 { return 0 }

// ------------------------------------------------------------
// (15) 下拉文件指令 PDU
// ------------------------------------------------------------

type DownloadFilePDU struct {
    BasePDU
    CommandID  uint64
    FileName   string
    Offset     uint16
    Signature  []byte
}

func (p *DownloadFilePDU) GetSessionID() uint32 { return 0 }

// ------------------------------------------------------------
// (16) 差分升级指令
// ------------------------------------------------------------

type DeltaUpgradePDU struct {
    BasePDU
    CommandID  uint64
    SoftwareID uint64
    TargetVer  uint32
    Signature  []byte
}

func (p *DeltaUpgradePDU) GetSessionID() uint32 { return 0 }

// ------------------------------------------------------------
// (17) 升级成功报告
// ------------------------------------------------------------

type UpgradeSuccessPDU struct {
    BasePDU
    CommandID uint64
    SoftwareID uint64
    NewVersion uint32
    Timestamp  uint64
}

func (p *UpgradeSuccessPDU) GetSessionID() uint32 { return 0 }

// ------------------------------------------------------------
// (18) 升级失败报告
// ------------------------------------------------------------

type UpgradeFailedPDU struct {
    BasePDU
    CommandID   uint64
    SoftwareID  uint64
    TargetVer   uint32
    FailStage   uint8
    ErrorCode   uint8
    Timestamp   uint64
}

func (p *UpgradeFailedPDU) GetSessionID() uint32 { return 0 }
