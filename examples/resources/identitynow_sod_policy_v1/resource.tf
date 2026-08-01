resource "identitynow_sod_policy_v1" "example" {
  name                     = "example-sod-policy"
  description              = "Managed by Terraform. Flags identities with conflicting finance access."
  external_policy_reference = "FIN-SOD-001"
  compensating_controls    = "Quarterly manager attestation review."
  correction_advice        = "Remove one of the conflicting entitlements or request a policy exception."
  state                    = "ENFORCED"
  scheduled                = false
  type                     = "CONFLICTING_ACCESS_BASED"
  tags                     = ["finance", "sox"]

  owner_ref = {
    type = "IDENTITY"
    id   = "2c9180835d191305015d28d181fc1234"
    name = "Support"
  }

  conflicting_access_criteria = {
    left_criteria = {
      name = "AP Invoice Approval"
      criteria_list = [
        {
          type = "ENTITLEMENT"
          id   = "2c9180846b0910c8016b0911d4a10101"
          name = "AP-Invoice-Approver"
        }
      ]
    }
    right_criteria = {
      name = "AP Payment Processing"
      criteria_list = [
        {
          type = "ENTITLEMENT"
          id   = "2c9180846b0910c8016b0911d4a10202"
          name = "AP-Payment-Processor"
        }
      ]
    }
  }

  violation_owner_assignment_config = {
    assignment_rule = "MANAGER"
  }
}
