<script setup>
import { Lock } from "lucide-vue-next";
import { onMounted, ref } from "vue";

import AppBadge from "@/components/ui/AppBadge.vue";
import AppButton from "@/components/ui/AppButton.vue";
import AppDialog from "@/components/ui/AppDialog.vue";
import ConfirmDialog from "@/components/ui/ConfirmDialog.vue";
import LoadingRows from "@/components/ui/LoadingRows.vue";
import FormField from "@/components/ui/FormField.vue";
import ToggleSwitch from "@/components/ui/ToggleSwitch.vue";
import api, { errorMessage, fieldErrors } from "@/lib/api";
import { useSession } from "@/stores/session";
import { useToast } from "@/stores/toast";

/**
 * A group's firewall rules.
 *
 * The server lists deny rules ahead of allow rules, which is the order it
 * writes them into the group's chain, and every chain ends in a drop. The
 * table shows that drop as a last, built-in row, so it reads top to bottom
 * as the firewall checks it.
 */
const props = defineProps({
  groupId: { type: String, required: true },
});

const emit = defineEmits(["count"]);

const session = useSession();
const toast = useToast();

const rules = ref([]);
const loaded = ref(false);

const adding = ref(false);
const draft = ref({});
const errors = ref({});
const busy = ref(false);
const removing = ref(null);

async function load() {
  const { data } = await api.get(`/admin/groups/${props.groupId}/firewall`);
  rules.value = data;
  loaded.value = true;
  emit("count", data.length);
}

/** Rule changes rewrite iptables; the server may now owe a restart. */
function settle() {
  session.refreshVpn().catch(() => {});
  return load();
}

function startAdding() {
  draft.value = { action: "ACCEPT", destination: "", protocol: "tcp", port: "" };
  errors.value = {};
  adding.value = true;
}

async function add() {
  busy.value = true;
  errors.value = {};
  try {
    // Ports only mean something once a protocol narrows the rule.
    const port = draft.value.protocol === "all" ? "" : draft.value.port;
    await api.post("/admin/firewall", { ...draft.value, port, group_id: props.groupId, is_enabled: true });
    adding.value = false;
    toast.success("Rule added.");
    await settle();
  } catch (error) {
    errors.value = fieldErrors(error);
    if (!Object.keys(errors.value).some((key) => key in draft.value)) toast.error(errorMessage(error));
  } finally {
    busy.value = false;
  }
}

async function setEnabled(rule, value) {
  const previous = rule.is_enabled;
  rule.is_enabled = value;
  try {
    await api.put(`/admin/firewall/${rule.id}`, { is_enabled: value });
    await settle();
  } catch (error) {
    rule.is_enabled = previous;
    toast.error(errorMessage(error, "The rule could not be changed."));
  }
}

// The page shows the Add rule button, beside its tabs.
defineExpose({ startAdding });

async function remove() {
  try {
    await api.delete(`/admin/firewall/${removing.value.id}`);
    toast.success("Rule deleted.");
    await settle();
  } catch (error) {
    toast.error(errorMessage(error, "The rule could not be deleted."));
    throw error;
  }
}

function protocol(rule) {
  return !rule.protocol || rule.protocol === "all" ? "All" : rule.protocol.toUpperCase();
}

onMounted(load);
</script>

<template>
  <!-- One root, so the parent can show and hide it. -->
  <div>
    <div class="card overflow-hidden">
      <LoadingRows v-if="!loaded" />

      <div v-else class="overflow-x-auto">
        <table class="w-full min-w-[680px] border-collapse">
          <thead>
            <tr>
              <th class="th">Action</th>
              <th class="th">Destination</th>
              <th class="th">Protocol</th>
              <th class="th">Ports</th>
              <th class="th">Enabled</th>
              <th class="th"><span class="sr-only">Actions</span></th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="rule in rules" :key="rule.id" :class="{ 'opacity-60': !rule.is_enabled }">
              <td class="td py-3">
                <AppBadge :tone="rule.action === 'ACCEPT' ? 'accent' : 'drop'">
                  {{ rule.action === "ACCEPT" ? "Allow" : "Deny" }}
                </AppBadge>
              </td>
              <td class="td py-3 font-mono text-[13px]">{{ rule.destination || "Anywhere" }}</td>
              <td class="td py-3 text-[13px] font-medium text-ink-2">{{ protocol(rule) }}</td>
              <td class="td py-3 font-mono text-[13px] text-ink-2">{{ rule.port || "Any" }}</td>
              <td class="td py-3">
                <ToggleSwitch
                  :disabled="!session.isAdmin"
                  :model-value="rule.is_enabled"
                  :label="`Rule for ${rule.destination || 'anywhere'} enabled`"
                  @update:model-value="(value) => setEnabled(rule, value)"
                />
              </td>
              <td class="td py-3 text-right">
                <button
                  v-if="session.isAdmin"
                  type="button"
                  aria-label="Delete rule"
                  class="inline-flex size-8 items-center justify-center rounded-lg text-ink-4 hover:bg-deny-soft hover:text-deny"
                  @click="removing = rule"
                >
                  <Trash2 class="size-4" :stroke-width="2" />
                </button>
              </td>
            </tr>            <!-- Every chain ends in a drop. It can't be changed, so it is
                 shown as part of the list rather than as a rule. -->
            <tr>
              <td class="td py-3">
                <AppBadge tone="drop">Deny</AppBadge>
              </td>
              <td class="td py-3">
                <span class="flex items-center gap-2">
                  <span class="font-mono text-[13px]">Anywhere</span>
                  <AppBadge>Built in</AppBadge>
                </span>
              </td>
              <td class="td py-3 text-[13px] font-medium text-ink-2">All</td>
              <td class="td py-3 font-mono text-[13px] text-ink-2">Any</td>
              <td class="td py-3">
                <span class="inline-flex items-center gap-1.5 text-[13px] text-ink-3" title="The last rule in every group. It can't be turned off.">
                  <Lock class="size-3.5" :stroke-width="2" />
                  Always on
                </span>
              </td>
              <td class="td py-3" />
            </tr>
          </tbody>
        </table>
      </div>
    </div>

    <AppDialog v-model:open="adding" title="Add firewall rule" :busy="busy">
      <form id="new-rule" class="flex flex-col gap-[18px]" @submit.prevent="add">
        <fieldset class="flex flex-col gap-2">
          <legend class="label mb-2">Action</legend>
          <div class="grid grid-cols-2 gap-2">
            <label
              v-for="option in [
                { value: 'ACCEPT', label: 'Allow' },
                { value: 'DROP', label: 'Deny' },
              ]"
              :key="option.value"
              class="flex h-11 items-center justify-center gap-2 rounded-lg border transition-colors"
              :class="
                draft.action === option.value
                  ? 'border-2 border-accent bg-accent-soft font-semibold text-accent-strong'
                  : 'border-field font-medium text-ink-2 hover:bg-subtle'
              "
            >
              <input v-model="draft.action" type="radio" name="action" :value="option.value" class="accent-accent" />
              {{ option.label }}
            </label>
          </div>
          <p class="hint">Deny rules are always checked before allow rules.</p>
          <p v-if="errors.action" class="field-error">{{ errors.action }}</p>
        </fieldset>

        <FormField
          v-slot="field"
          label="Destination"
          hint="An IPv4 address or CIDR range. Leave blank for anywhere."
          :error="errors.destination"
        >
          <input
            :id="field.id"
            v-model="draft.destination"
            type="text"
            autofocus
            placeholder="10.0.10.0/24"
            class="input font-mono text-[13px]"
            :aria-invalid="field.invalid"
            :aria-describedby="field.describedby"
          />
        </FormField>

        <div class="grid gap-3.5 sm:grid-cols-[160px_minmax(0,1fr)]">
          <FormField v-slot="field" label="Protocol" :error="errors.protocol">
            <select :id="field.id" v-model="draft.protocol" class="input" :aria-invalid="field.invalid" :aria-describedby="field.describedby">
              <option value="all">All</option>
              <option value="tcp">TCP</option>
              <option value="udp">UDP</option>
            </select>
          </FormField>

          <FormField
            v-slot="field"
            label="Ports"
            hint="e.g. 443, 22,80 or 8000:8100. Blank for any."
            :error="errors.port"
          >
            <input
              :id="field.id"
              v-model="draft.port"
              type="text"
              :disabled="draft.protocol === 'all'"
              class="input font-mono text-[13px]"
              :aria-invalid="field.invalid"
              :aria-describedby="field.describedby"
            />
          </FormField>
        </div>
      </form>

      <template #footer>
        <AppButton :disabled="busy" @click="adding = false">Cancel</AppButton>
        <AppButton variant="primary" type="submit" form="new-rule" :disabled="busy">
          {{ busy ? "Adding…" : "Add rule" }}
        </AppButton>
      </template>
    </AppDialog>

    <ConfirmDialog
      :open="removing !== null"
      title="Delete this rule?"
      :message="`${removing?.action === 'ACCEPT' ? 'Allow' : 'Deny'} ${removing?.destination || 'anywhere'} is removed from the group's firewall.`"
      confirm-label="Delete rule"
      :action="remove"
      @update:open="(value) => !value && (removing = null)"
    />
  </div>
</template>
