package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"sync"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	computepb "google.golang.org/genproto/googleapis/cloud/compute/v1"
)

var (
	project     = "ajackura-dev"
	zone        = "us-central1-a"
	machineType = "zones/us-central1-a/machineTypes/e2-standard-2"
	diskType    = fmt.Sprintf("zones/%s/diskTypes/pd-balanced", zone)
	image       = "global/images/agile-os-v1642653211"
	//image = "global/images/deb-benchmark"
)

func stringP(s string) *string { return &s }

func boolP(b bool) *bool { return &b }

func opWait(ctx context.Context, zOpClient *compute.ZoneOperationsClient, op *computepb.Operation) error {
	var err error
	for {
		if op.GetStatus() == computepb.Operation_DONE {
			break
		}
		op, err = zOpClient.Wait(ctx, &computepb.WaitZoneOperationRequest{Operation: op.GetName(), Project: project, Zone: zone})
		if err != nil {
			return err
		}
	}
	return nil
}

func createInstance(ctx context.Context, instClient *compute.InstancesClient, diskClient *compute.DisksClient, zOpClient *compute.ZoneOperationsClient, name string) (string, string, string, string) {
	start := time.Now()
	diskReq := &computepb.InsertDiskRequest{
		Project:     project,
		Zone:        zone,
		SourceImage: &image,
		DiskResource: &computepb.Disk{
			Name:   &name,
			SizeGb: func() *int64 { i := int64(10); return &i }(),
			Type:   &diskType,
		},
	}
	o, err := diskClient.Insert(ctx, diskReq)
	if err != nil {
		log.Print(err)
		return "", "", "", ""
	}
	if err := opWait(ctx, zOpClient, o.Proto()); err != nil {
		log.Print(err)
		return "", "", "", ""
	}

	diskTime := time.Since(start).String()
	start = time.Now()

	req := &computepb.InsertInstanceRequest{
		Project: project,
		Zone:    zone,
		InstanceResource: &computepb.Instance{
			MachineType: &machineType,
			Name:        &name,
			Disks: []*computepb.AttachedDisk{
				{
					AutoDelete: boolP(true),
					Boot:       boolP(true),
					Source:     stringP(fmt.Sprintf("zones/%s/disks/%s", zone, name)),
					//	InitializeParams: &computepb.AttachedDiskInitializeParams{
					//		DiskSizeGb:  func() *int64 { i := int64(10); return &i }(),
					//		DiskType:    stringP(fmt.Sprintf("zones/%s/diskTypes/pd-ssd", zone)),
					//		SourceImage: &image,
					//	},
				},
			},
			NetworkInterfaces: []*computepb.NetworkInterface{
				{
					Network: stringP("global/networks/default"),
					AccessConfigs: []*computepb.AccessConfig{
						{
							Name:        stringP("External NAT"),
							NetworkTier: stringP("Premium"),
						},
					},
				},
			},
			Tags: &computepb.Tags{
				Items: []string{"http-server"},
			},
			Metadata: &computepb.Metadata{
				Items: []*computepb.Items{
					{
						Key:   stringP("osconfig-log-level"),
						Value: stringP("info"),
					},
				},
			},
		},
	}
	o, err = instClient.Insert(ctx, req)
	if err != nil {
		log.Print(err)
		return "", "", "", ""
	}
	if err := opWait(ctx, zOpClient, o.Proto()); err != nil {
		log.Print(err)
		return "", "", "", ""
	}

	inst, err := instClient.Get(ctx, &computepb.GetInstanceRequest{Instance: name, Project: project, Zone: zone})
	if err != nil {
		log.Print(err)
		return "", "", "", ""
	}
	defer instClient.Delete(ctx, &computepb.DeleteInstanceRequest{Instance: name, Project: project, Zone: zone})
	ip := inst.GetNetworkInterfaces()[0].GetAccessConfigs()[0].GetNatIP()
	//fmt.Println(time.Since(start).Seconds(), "instance ip:", ip)

	insertTime := time.Since(start).String()
	start = time.Now()

	htc := &http.Client{Timeout: 100 * time.Millisecond}
	addr := fmt.Sprintf("http://%s:8080/hello", ip)
	for {
		_, err := htc.Get(addr)
		if err != nil {
			//log.Print(err)
			time.Sleep(10 * time.Millisecond)
			continue
		}
		break
	}
	serveTime := time.Since(start).String()

	out, err := instClient.GetSerialPortOutput(ctx, &computepb.GetSerialPortOutputInstanceRequest{Instance: name, Project: project, Zone: zone, Port: func() *int32 { i := int32(1); return &i }()})
	if err != nil {
		log.Print(err)
		return "", "", "", ""
	}
	var kernelTime string
	for _, line := range strings.Split(out.GetContents(), "\n") {
		if strings.Contains(line, "Hello World!") {
			kernelTime = line
		}
	}

	return diskTime, insertTime, serveTime, kernelTime
}

func main() {
	ctx := context.Background()

	instClient, err := compute.NewInstancesRESTClient(ctx)
	//service, err := compute.NewService(ctx)
	if err != nil {
		log.Fatal(err)
	}

	diskClient, err := compute.NewDisksRESTClient(ctx)
	//service, err := compute.NewService(ctx)
	if err != nil {
		log.Fatal(err)
	}

	zOpClient, err := compute.NewZoneOperationsRESTClient(ctx)
	//service, err := compute.NewService(ctx)
	if err != nil {
		log.Fatal(err)
	}

	var wg sync.WaitGroup
	for i := 1; i < 6; i++ {
		wg.Add(1)
		go func(i int) {
			t1, t2, t3, t4 := createInstance(ctx, instClient, diskClient, zOpClient, fmt.Sprintf("benchmark-%d", i))
			fmt.Println(t1, t2, t3, t4)
			wg.Done()
		}(i)
	}
	wg.Wait()
}
