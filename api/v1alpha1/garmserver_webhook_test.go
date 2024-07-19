// SPDX-License-Identifier: MIT

package v1alpha1

import (
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("GarmServer Webhook", func() {

	Context("When creating GarmServer under Defaulting Webhook", func() {
		It("Should fill in the default value if a required field is empty", func() {

			// TODO(user): Add your logic here

		})
	})

	Context("When creating GarmServer under Validating Webhook", func() {
		It("Should deny if a required field is empty", func() {

			// TODO(user): Add your logic here

		})

		It("Should admit if all required fields are provided", func() {

			// TODO(user): Add your logic here

		})
	})

})
