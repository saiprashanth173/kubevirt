/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright 2020 Red Hat, Inc.
 *
 */

package sriov

import (
	"fmt"
	"time"

	v1 "kubevirt.io/api/core/v1"

	"kubevirt.io/kubevirt/pkg/network/sriov"
	"kubevirt.io/kubevirt/pkg/network/vmispec"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/api"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/device"
	"kubevirt.io/kubevirt/pkg/virt-launcher/virtwrap/device/hostdevice"
)

func CreateHostDevices(vmi *v1.VirtualMachineInstance) ([]api.HostDevice, error) {
	SRIOVInterfaces := vmispec.FilterSRIOVInterfaces(vmi.Spec.Domain.Devices.Interfaces)
	return CreateHostDevicesFromIfacesAndPool(SRIOVInterfaces, NewPCIAddressPool(SRIOVInterfaces))
}

func CreateHostDevicesFromIfacesAndPool(ifaces []v1.Interface, pool hostdevice.AddressPooler) ([]api.HostDevice, error) {
	hostDevicesMetaData := createHostDevicesMetadata(ifaces)
	return hostdevice.CreatePCIHostDevices(hostDevicesMetaData, pool)
}

func createHostDevicesMetadata(ifaces []v1.Interface) []hostdevice.HostDeviceMetaData {
	var hostDevicesMetaData []hostdevice.HostDeviceMetaData
	for _, iface := range ifaces {
		hostDevicesMetaData = append(hostDevicesMetaData, hostdevice.HostDeviceMetaData{
			AliasPrefix:  sriov.AliasPrefix,
			Name:         iface.Name,
			ResourceName: iface.Name,
			DecorateHook: newDecorateHook(iface),
		})
	}
	return hostDevicesMetaData
}

func newDecorateHook(iface v1.Interface) func(hostDevice *api.HostDevice) error {
	return func(hostDevice *api.HostDevice) error {
		if guestPCIAddress := iface.PciAddress; guestPCIAddress != "" {
			addr, err := device.NewPciAddressField(guestPCIAddress)
			if err != nil {
				return fmt.Errorf("failed to interpret the guest PCI address: %v", err)
			}
			hostDevice.Address = addr
		}

		if iface.BootOrder != nil {
			hostDevice.BootOrder = &api.BootOrder{Order: *iface.BootOrder}
		}
		return nil
	}
}

func SafelyDetachHostDevices(domainSpec *api.DomainSpec, eventDetach hostdevice.EventRegistrar, dom hostdevice.DeviceDetacher, timeout time.Duration) error {
	sriovDevices := hostdevice.FilterHostDevicesByAlias(domainSpec.Devices.HostDevices, sriov.AliasPrefix)
	return hostdevice.SafelyDetachHostDevices(sriovDevices, eventDetach, dom, timeout)
}

func GetHostDevicesToAttach(vmi *v1.VirtualMachineInstance, domainSpec *api.DomainSpec) ([]api.HostDevice, error) {
	sriovDevices, err := CreateHostDevices(vmi)
	if err != nil {
		return nil, err
	}
	currentAttachedSRIOVHostDevices := hostdevice.FilterHostDevicesByAlias(domainSpec.Devices.HostDevices, sriov.AliasPrefix)

	sriovHostDevicesToAttach := hostdevice.DifferenceHostDevicesByAlias(sriovDevices, currentAttachedSRIOVHostDevices)

	return sriovHostDevicesToAttach, nil
}
