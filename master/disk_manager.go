package master

import (
	"fmt"
	"github.com/tiglabs/containerfs/util"
	"github.com/tiglabs/containerfs/util/log"
	"math"
	"time"
)

func (c *Cluster) startCheckBadDiskRecovery() {
	go func() {
		for {
			if c.partition != nil && c.partition.IsLeader() {
				if c.vols != nil {
					c.checkBadDiskRecovery()
				}
			}
			time.Sleep(time.Second * DefaultCheckDataPartitionIntervalSeconds)
		}
	}()
}

func (c *Cluster) checkBadDiskRecovery() {
	var minus float64
	c.BadDataPartitionIds.Range(func(key, value interface{}) bool {
		badDataPartitionIds := value.([]uint64)
		newBadDpIds := make([]uint64, 0)
		for _, partitionID := range badDataPartitionIds {
			partition, err := c.getDataPartitionByID(partitionID)
			if err != nil {
				continue
			}
			vol, err := c.getVol(partition.VolName)
			if err != nil {
				continue
			}
			if len(partition.Replicas) == 0 || len(partition.Replicas) < int(vol.dpReplicaNum) {
				continue
			}
			used := partition.Replicas[0].Used
			for _, replica := range partition.Replicas {
				if math.Abs(float64(replica.Used)-float64(used)) > minus {
					minus = math.Abs(float64(replica.Used) - float64(used))
				}
			}
			if minus < util.GB {
				Warn(c.Name, fmt.Sprintf("clusterID[%v],partitionID[%v] has recovered success", c.Name, partitionID))
				c.BadDataPartitionIds.Delete(key)
			} else {
				newBadDpIds = append(newBadDpIds, partitionID)
			}
		}
		if len(newBadDpIds) == 0 {
			Warn(c.Name, fmt.Sprintf("clusterID[%v],node:disk[%v] has recovered success", c.Name, key))
		} else {
			c.BadDataPartitionIds.Store(key, newBadDpIds)
		}

		return true
	})
}

func (c *Cluster) diskOffLine(dataNode *DataNode, badDiskPath string, badPartitionIds []uint64) (err error) {
	msg := fmt.Sprintf("action[diskOffLine], Node[%v] OffLine,disk[%v]", dataNode.Addr, badDiskPath)
	log.LogWarn(msg)
	safeVols := c.getAllNormalVols()
	for _, vol := range safeVols {
		for _, dp := range vol.dataPartitions.dataPartitions {
			for _, bad := range badPartitionIds {
				if bad == dp.PartitionID {
					if err = c.dataPartitionOffline(dataNode.Addr, vol.Name, dp, DiskOfflineInfo); err != nil {
						return
					}
				}
			}
		}
	}
	msg = fmt.Sprintf("action[diskOffLine],clusterID[%v] Node[%v] OffLine success",
		c.Name, dataNode.Addr)
	Warn(c.Name, msg)
	return
}