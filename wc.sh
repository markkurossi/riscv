#!/bin/sh

isa=`wc -l isa/*.go | tail -1 | awk '{print $1}'`
mem=`wc -l memory/*.go mmu/*.go | tail -1 | awk '{print $1}'`
dev=`wc -l dev/*.go virtio/*.go | tail -1 | awk '{print $1}'`
orc=`wc -l cmd/goemu/main*.go cmd/goemu/system_*.go | tail -1 | awk '{print $1}'`
cpu=`wc -l cpu/*.go | tail -1 | awk '{print $1}'`
vpu=`wc -l cpu/vpu.go | tail -1 | awk '{print $1}'`
cpu_base=`expr $cpu - $vpu`

total=`expr $isa + $mem + $dev + $orc + $cpu`

echo "ISA                :  $isa"
echo "Memory and MMU     :  $mem"
echo "CPU                :  $cpu"
echo " - Base            :  $cpu_base"
echo " - VPU             :  $vpu"
echo "Devices and VirtIO :  $dev"
echo "Orchestration      :  $orc"
echo "=========================="
echo "Total              : $total"
