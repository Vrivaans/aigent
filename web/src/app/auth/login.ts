import { Component, signal, inject } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { AuthService } from './auth.service';
import { ApiService } from '../api.service';
import { TranslationService } from '../translation.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './login.html',
  styleUrls: ['./login.css']
})
export class LoginComponent {
  private auth = inject(AuthService);
  private api = inject(ApiService);
  private router = inject(Router);
  private translation = inject(TranslationService);

  t(key: string, params?: Record<string, string>): string {
    return this.translation.t(key, params);
  }

  username = signal('');
  password = signal('');
  error = signal('');
  isLoading = signal(false);

  async onLogin() {
    if (!this.username() || !this.password()) return;

    this.isLoading.set(true);
    this.error.set('');

    try {
      const resp = await this.api.login(this.username(), this.password());
      this.auth.setToken(resp.token);
      this.router.navigate(['/chat']);
    } catch (e: any) {
      this.error.set(this.t('login.error_credentials'));
    } finally {
      this.isLoading.set(false);
    }
  }
}
