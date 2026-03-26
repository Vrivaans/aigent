import { Component, signal } from '@angular/core';
import { CommonModule } from '@angular/common';
import { FormsModule } from '@angular/forms';
import { Router } from '@angular/router';
import { AuthService } from './auth.service';
import { ApiService } from '../api.service';

@Component({
  selector: 'app-login',
  standalone: true,
  imports: [CommonModule, FormsModule],
  templateUrl: './login.html',
  styleUrls: ['./login.css']
})
export class LoginComponent {
  username = signal('');
  password = signal('');
  error = signal('');
  isLoading = signal(false);

  constructor(
    private auth: AuthService,
    private api: ApiService,
    private router: Router
  ) {}

  async onLogin() {
    if (!this.username() || !this.password()) return;

    this.isLoading.set(true);
    this.error.set('');

    try {
      const resp = await this.api.login(this.username(), this.password());
      this.auth.setToken(resp.token);
      this.router.navigate(['/chat']);
    } catch (e: any) {
      this.error.set('Credenciales incorrectas o error de servidor');
    } finally {
      this.isLoading.set(false);
    }
  }
}
