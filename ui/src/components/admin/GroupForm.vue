<script setup>
import FormField from "@/components/ui/FormField.vue";
import SettingRow from "@/components/ui/SettingRow.vue";
import SwitchRow from "@/components/ui/SwitchRow.vue";
import ToggleSwitch from "@/components/ui/ToggleSwitch.vue";

/**
 * The fields a group has, shared by creating one and editing one. The
 * model is the group being edited; `errors` are the server's, by field.
 *
 * Creating asks only for what a new group needs, stacked for a dialog.
 * `full` shows every setting as a row, name on the left and control on the
 * right, for the group's own page.
 */
defineProps({
  errors: { type: Object, default: () => ({}) },
  full: { type: Boolean, default: false },
});

const group = defineModel({ type: Object, required: true });

const IDLE_CHOICES = [
  { value: 0, label: "Never" },
  { value: 30, label: "After 30 days unused" },
  { value: 60, label: "After 60 days unused" },
  { value: 90, label: "After 90 days unused" },
  { value: 180, label: "After 180 days unused" },
  { value: 365, label: "After a year unused" },
];
</script>

<template>
  <div v-if="full" class="flex flex-col">
    <SettingRow v-slot="field" label="Name" :error="errors.name">
      <input :id="field.id" v-model="group.name" type="text" maxlength="32" class="input" :aria-invalid="field.invalid" :aria-describedby="field.describedby" />
    </SettingRow>

    <SettingRow v-slot="field" label="Description" :error="errors.description">
      <input :id="field.id" v-model="group.description" type="text" class="input" :aria-invalid="field.invalid" :aria-describedby="field.describedby" />
    </SettingRow>

    <SettingRow label="Allow sign-in" description="When off, members can't sign in or connect, and anyone connected is disconnected.">
      <ToggleSwitch v-model="group.is_enabled" label="Allow sign-in" class="sm:mt-2" />
    </SettingRow>

    <SettingRow label="Require two-factor" description="Members must use an authenticator app, unless set otherwise on their account.">
      <ToggleSwitch v-model="group.mfa_enforced" label="Require two-factor" class="sm:mt-2" />
    </SettingRow>

    <SettingRow v-slot="field" label="Devices per user" description="How many VPN devices each member may create." :error="errors.max_devices">
      <input :id="field.id" v-model.number="group.max_devices" type="number" min="1" class="input w-28" :aria-invalid="field.invalid" :aria-describedby="field.describedby" />
    </SettingRow>

    <SettingRow v-slot="field" label="Remove unused devices" description="Their keys are revoked; owners can add them again." :error="errors.device_idle_days">
      <select :id="field.id" v-model.number="group.device_idle_days" class="input" :aria-invalid="field.invalid" :aria-describedby="field.describedby">
        <option v-for="choice in IDLE_CHOICES" :key="choice.value" :value="choice.value">{{ choice.label }}</option>
      </select>
    </SettingRow>

    <SettingRow v-slot="field" label="Routes" description="Networks members can reach, added to the server's own. One per line." :error="errors.routes">
      <textarea :id="field.id" v-model="group.routes" rows="3" placeholder="10.0.10.0/24" class="input font-mono text-[13px]" :aria-invalid="field.invalid" :aria-describedby="field.describedby" />
    </SettingRow>

    <SettingRow v-slot="field" label="DNS servers" description="Added to the server's own. One per line." :error="errors.nameservers">
      <textarea :id="field.id" v-model="group.nameservers" rows="3" placeholder="10.0.0.53" class="input font-mono text-[13px]" :aria-invalid="field.invalid" :aria-describedby="field.describedby" />
    </SettingRow>
  </div>

  <div v-else class="flex flex-col gap-[18px]">
    <FormField v-slot="field" label="Name" :error="errors.name">
      <input :id="field.id" v-model="group.name" type="text" maxlength="32" autofocus class="input" :aria-invalid="field.invalid" :aria-describedby="field.describedby" />
    </FormField>
    <FormField v-slot="field" label="Description" :error="errors.description">
      <input :id="field.id" v-model="group.description" type="text" class="input" :aria-invalid="field.invalid" :aria-describedby="field.describedby" />
    </FormField>
    <FormField v-slot="field" label="Devices per user" hint="How many VPN devices each member may create." :error="errors.max_devices">
      <input :id="field.id" v-model.number="group.max_devices" type="number" min="1" class="input w-28" :aria-invalid="field.invalid" :aria-describedby="field.describedby" />
    </FormField>
    <SwitchRow v-model="group.mfa_enforced" label="Require two-factor" description="Members must use an authenticator app, unless set otherwise on their account." />
  </div>
</template>
