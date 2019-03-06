from rest_framework import permissions


class AdminPermission(permissions.BasePermission):
    def has_permission(self, request, view):
        """
        Returns True if the User is an administrator.
        :return: bool
        """
        return request.user.is_active and request.user.is_admin

    def has_object_permission(self, request, view, obj):
        """
        Returns True since Admins have access to everything.
        :return: bool
        """
        return True
