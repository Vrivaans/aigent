import { Component, inject, OnInit, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { ApiService, AppRole, AppUser } from '../api.service';
import { TranslationService } from '../translation.service';

@Component({
  selector: 'app-users',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './users.html',
  styleUrl: './users.css'
})
export class UsersComponent implements OnInit {
  private api = inject(ApiService);
  private translation = inject(TranslationService);

  t(key: string, params?: Record<string, string>): string {
    return this.translation.t(key, params);
  }

  users = signal<AppUser[]>([]);
  roles = signal<AppRole[]>([]);
  isLoading = signal(false);
  isSaving = signal(false);
  showCreateForm = signal(false);

  newUser = {
    username: '',
    password: '',
    role: 'operator'
  };

  roleEdits = signal<Record<number, string>>({});

  async ngOnInit() {
    await this.loadData();
  }

  async loadData() {
    this.isLoading.set(true);
    try {
      const [users, roles] = await Promise.all([
        this.api.getAdminUsers(),
        this.api.getAdminRoles()
      ]);
      this.users.set(users);
      this.roles.set(roles);
      const edits: Record<number, string> = {};
      for (const u of users) {
        edits[u.id] = u.roles[0] ?? 'viewer';
      }
      this.roleEdits.set(edits);
    } catch (err) {
      console.error('Failed to load users:', err);
      alert(this.t('users.error_load'));
    } finally {
      this.isLoading.set(false);
    }
  }

  toggleCreateForm() {
    this.showCreateForm.update(v => !v);
    if (!this.showCreateForm()) {
      this.newUser = { username: '', password: '', role: 'operator' };
    }
  }

  async createUser() {
    const username = this.newUser.username.trim();
    const password = this.newUser.password;
    if (!username || !password) {
      alert(this.t('users.error_required'));
      return;
    }

    this.isSaving.set(true);
    try {
      await this.api.createAdminUser({
        username,
        password,
        roles: [this.newUser.role]
      });
      this.showCreateForm.set(false);
      this.newUser = { username: '', password: '', role: 'operator' };
      await this.loadData();
    } catch (err) {
      console.error('Failed to create user:', err);
      alert(this.t('users.error_create'));
    } finally {
      this.isSaving.set(false);
    }
  }

  setRoleEdit(userId: number, role: string) {
    this.roleEdits.update(edits => ({ ...edits, [userId]: role }));
  }

  async saveUserRoles(user: AppUser) {
    const role = this.roleEdits()[user.id];
    if (!role || user.roles.includes(role) && user.roles.length === 1) {
      return;
    }

    this.isSaving.set(true);
    try {
      const updated = await this.api.updateAdminUserRoles(user.id, [role]);
      this.users.update(list => list.map(u => u.id === updated.id ? updated : u));
    } catch (err) {
      console.error('Failed to update user roles:', err);
      alert(this.t('users.error_roles'));
    } finally {
      this.isSaving.set(false);
    }
  }

  rolesLabel(user: AppUser): string {
    return user.roles.join(', ');
  }
}
