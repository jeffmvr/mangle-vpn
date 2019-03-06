<template>
  <tr class="form-table-row">
    <td>
      <b><slot name="label"></slot></b>
      <p class="help">
        <slot name="help"></slot>
      </p>
    </td>
    <td>
      <div class="field">
        <slot name="input"></slot>
      </div>
    </td>
  </tr>
</template>

<script>
  export default {
    name: "FormTableRow"
  }
</script>

<style>
  tr.form-table-row td > p.help {
    color: #888;
    margin-top: 0.4rem;
  }
  tr.form-table-row td:first-child {
    width: 55%;
  }
  tr.form-table-row td:last-child {
    vertical-align: middle;
  }
</style>
