"""统一异常体系."""


class DevConnectError(Exception):
    """基础异常."""


class ConfigError(DevConnectError):
    """配置错误."""


class HostNotFoundError(ConfigError):
    """主机未找到."""


class SSHError(DevConnectError):
    """SSH 连接错误."""


class CommandError(SSHError):
    """远程命令执行错误."""


class TransferError(SSHError):
    """文件传输错误."""


class TimeoutError(SSHError):
    """操作超时."""
