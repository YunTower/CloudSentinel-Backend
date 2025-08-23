package migrations

import (
	"github.com/goravel/framework/contracts/database/schema"
	"github.com/goravel/framework/facades"
)

type M20241207000005CreateServerDisksTable struct {
}

// Signature The unique signature for the migration.
func (r *M20241207000005CreateServerDisksTable) Signature() string {
	return "20241207000005_create_server_disks_table"
}

// Up Run the migrations.
func (r *M20241207000005CreateServerDisksTable) Up() error {
	if !facades.Schema().HasTable("server_disks") {
		return facades.Schema().Create("server_disks", func(table schema.Blueprint) {
			table.ID()
			table.String("server_id", 255) // 关联的服务器ID
			table.String("disk_name", 100) // 磁盘名称（如/dev/sda, /dev/nvme0n1）
			table.String("mount_point", 200).Nullable() // 挂载点（如/, /home, /data）
			table.String("filesystem", 50).Nullable() // 文件系统类型（如ext4, xfs, ntfs）
			table.BigInteger("total_size") // 磁盘总容量（字节）
			table.BigInteger("used_size").Default(0) // 已使用容量（字节）
			table.BigInteger("free_size").Default(0) // 可用容量（字节）
			table.String("disk_type", 50).Default("unknown") // 磁盘类型（如ssd, hdd, nvme）
			table.Boolean("is_boot").Default(false) // 是否为启动磁盘
			table.Timestamps()
			
			// 外键约束
			table.Foreign("server_id").References("id").On("servers")
		})
	}

	return nil
}

// Down Reverse the migrations.
func (r *M20241207000005CreateServerDisksTable) Down() error {
	return facades.Schema().DropIfExists("server_disks")
}
